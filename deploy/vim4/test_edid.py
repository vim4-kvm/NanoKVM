import hashlib
import importlib.machinery
import importlib.util
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

loader = importlib.machinery.SourceFileLoader('edid', str(Path(__file__).with_name('vim4-edid')))
spec = importlib.util.spec_from_loader(loader.name, loader)
edid = importlib.util.module_from_spec(spec)
loader.exec_module(edid)


def block(extensions=0, tail=b''):
    data = bytearray(bytes([0, 255, 255, 255, 255, 255, 255, 0]) + bytes(118) + bytes([extensions, 0]))
    data[127] = -sum(data) % 256
    return bytes(data) + tail


def extension():
    data = bytearray(128)
    data[0] = 2
    data[127] = -sum(data) % 256
    return bytes(data)


class EdidTest(unittest.TestCase):
    def setUp(self):
        self.enterContext(patch.object(edid.platform, 'release', return_value='5.15-test'))
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        state = patch.object(edid, 'STATE', Path(self.directory.name))
        lock = patch.object(edid, 'LOCK', str(Path(self.directory.name) / 'lock'))
        loaded = patch.object(edid, 'LOADED', Path(self.directory.name) / 'loaded')
        run = patch.object(edid, 'tvserver_run', return_value='run-1')
        select = Path(self.directory.name) / 'edid_select'
        select.write_text('edid select for portD~A: 0x10\n')
        selected = patch.object(edid, 'EDID_SELECT', select)
        for patcher in (state, lock, loaded, run, selected):
            patcher.start()
            self.addCleanup(patcher.stop)

    def test_validation(self):
        self.assertEqual(len(edid.validate(block())), 256)
        self.assertEqual(edid.validate(block(1, extension())), block(1, extension()))
        broken = bytearray(block())
        broken[20] ^= 1
        for value in (b'', block()[:127], block(1), block(0, extension()), bytes(broken), block(2, extension() * 2)):
            with self.subTest(size=len(value)), self.assertRaises(ValueError):
                edid.validate(value)

    def test_failed_load_is_not_persisted(self):
        with patch.object(edid, 'load', side_effect=OSError('ioctl failed')), self.assertRaises(OSError):
            edid.apply('custom.bin', block())
        self.assertEqual(edid.active(), (None, None))

    def test_default_preset_clears_boot_override(self):
        edid.persist('custom.bin', block())
        with patch.object(edid, 'load') as load:
            edid.apply(edid.DEFAULT, block())
        load.assert_called_once()
        self.assertEqual(edid.active(), (None, None))

    def test_invalid_requests_do_not_load(self):
        with patch.object(edid, 'load') as load:
            for args in (['apply', '../x'], ['apply', 'VIM4-4K60'], ['preset', 'custom'], ['apply'], ['status', 'x']):
                with self.subTest(args=args), self.assertRaises(ValueError):
                    edid.main(args)
        load.assert_not_called()

    def test_restore_waits_for_tvserver_then_loads_persisted(self):
        edid.persist('custom.bin', block())
        selections = iter(['edid select for portD~A: 0x222\n', 'edid select for portD~A: 0x222\n',
                           'edid select for portD~A: 0x0\n'])
        select = unittest.mock.Mock()
        select.read_text.side_effect = lambda: next(selections)
        with patch.object(edid, 'EDID_SELECT', select), patch.object(edid, 'load') as load, \
                patch.object(edid.time, 'sleep'):
            edid.restore()
        load.assert_called_once_with(edid.validate(block()))
        self.assertEqual(select.read_text.call_count, 3)
        self.assertEqual(edid.active()[0], 'custom.bin')

    def test_restore_once_per_tvserver_run(self):
        edid.persist('custom.bin', block())
        edid.LOADED.write_text(f'run-1 {edid.validate(block()).hex()}\n')
        with patch.object(edid, 'load') as load:
            edid.restore()
            load.assert_not_called()
            edid.tvserver_run.return_value = 'run-2'
            edid.restore()
        load.assert_called_once()

    def test_restore_without_selection_leaves_vendor_edid(self):
        with patch.object(edid, 'load') as load:
            edid.restore()
            edid.restore(reclaim=True)
        load.assert_not_called()

    def test_reclaim_replaces_another_applications_table(self):
        edid.persist('custom.bin', block())
        edid.LOADED.write_text('run-1 00ff\n')
        with patch.object(edid, 'load') as load:
            edid.restore()
            load.assert_not_called()
            edid.restore(reclaim=True)
        load.assert_called_once_with(edid.validate(block()))

    def test_reclaim_returns_default_and_skips_current_table(self):
        default = Path(self.directory.name) / 'default.bin'
        default.write_bytes(block(1, extension()))
        with patch.dict(edid.PRESETS, {edid.DEFAULT: default}), patch.object(edid, 'load') as load:
            edid.LOADED.write_text('run-1 00ff\n')
            edid.restore(reclaim=True)
            load.assert_called_once_with(block(1, extension()))
            edid.LOADED.write_text(f'run-1 {block(1, extension()).hex()}\n')
            edid.restore(reclaim=True)
        load.assert_called_once()

    def test_rtk_preset_is_the_board_copy_and_persists(self):
        # SHA256 recorded with the original in the vim4-kvm board/edid directory.
        self.assertEqual(hashlib.sha256(edid.validate(edid.RTK_UHD_HDR_EDID)).hexdigest(),
                         '32c6a60823dd676e848f8423feff1b469d07f5bac52885723dfa70b74cfda33d')
        with patch.object(edid, 'load') as load:
            edid.main(['preset', 'RTK-CONTROL'])
        load.assert_called_once_with(edid.RTK_UHD_HDR_EDID)
        self.assertEqual(edid.active(), ('RTK-CONTROL', edid.RTK_UHD_HDR_EDID))

    def test_rtk_name_only_accepts_the_rtk_table(self):
        stdin = unittest.mock.Mock()
        with patch.object(edid, 'load') as load, patch.object(edid.sys, 'stdin', stdin):
            stdin.buffer.read.return_value = block()
            with self.assertRaises(ValueError):
                edid.main(['apply', 'RTK-CONTROL'])
            load.assert_not_called()
            stdin.buffer.read.return_value = edid.RTK_UHD_HDR_EDID
            edid.main(['apply', 'RTK-CONTROL'])
        load.assert_called_once_with(edid.RTK_UHD_HDR_EDID)


if __name__ == '__main__':
    unittest.main()
