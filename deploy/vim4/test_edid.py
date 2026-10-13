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
        edid.LOADED.write_text('run-1\n')
        with patch.object(edid, 'load') as load:
            edid.restore()
            load.assert_not_called()
            edid.tvserver_run.return_value = 'run-2'
            edid.restore()
        load.assert_called_once()

    def test_restore_without_selection_leaves_vendor_edid(self):
        with patch.object(edid, 'load') as load:
            edid.restore()
        load.assert_not_called()


if __name__ == '__main__':
    unittest.main()
