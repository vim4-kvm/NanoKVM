import importlib.machinery
import importlib.util
import os
import sys
import fcntl
import io
import json
from contextlib import redirect_stdout
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

board_spec = importlib.util.spec_from_file_location('vim4media', Path(__file__).resolve().parents[3] / 'board/vim4media.py')
vim4media = importlib.util.module_from_spec(board_spec)
sys.modules['vim4media'] = vim4media
board_spec.loader.exec_module(vim4media)

loader = importlib.machinery.SourceFileLoader('media', str(Path(__file__).with_name('vim4-media')))
spec = importlib.util.spec_from_loader(loader.name, loader)
media = importlib.util.module_from_spec(spec)
loader.exec_module(media)


class MediaTest(unittest.TestCase):
    def test_service_restore_does_not_reconnect_ejected_or_replace_mounted_media(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'enabled').touch()
            (root / 'disk.img').write_bytes(b'existing user data')
            with patch.multiple(media, USB_ENABLED=root / 'enabled', USB_IMAGE=root / 'disk.img'), \
                    patch.object(media, 'configure') as configure, \
                    patch.object(media, 'usb_disk_fd') as disk:
                media.main(['disk', 'restore'])
                configure.assert_not_called()
                disk.assert_not_called()
                self.assertEqual((root / 'disk.img').read_bytes(), b'existing user data')

    @unittest.skipUnless(hasattr(os, 'posix_fallocate') and os.geteuid() == 0, 'Linux root helper')
    def test_usb_disk_is_created_once_and_failed_format_leaves_no_image(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            with patch.multiple(media, USB_DIR=root, USB_IMAGE=root / 'disk.img', USB_SIZE=1024 * 1024), \
                    patch.object(media.vim4media, 'DATA_DIR', root):
                with patch.object(media.subprocess, 'run', side_effect=OSError('format failed')):
                    with self.assertRaises(OSError):
                        media.usb_disk_fd()
                self.assertEqual(list(root.iterdir()), [])
                with patch.object(media.subprocess, 'run') as formatter:
                    fd = media.usb_disk_fd()
                    os.write(fd, b'host-written-data')
                    os.close(fd)
                    fd = media.usb_disk_fd()
                    try:
                        self.assertEqual(os.read(fd, 17), b'host-written-data')
                    finally:
                        os.close(fd)
                    formatter.assert_called_once()
                    self.assertEqual(formatter.call_args.args[0][3:5], ['-n', 'Vim4KVM'])

    @unittest.skipUnless(hasattr(os, 'posix_fallocate') and os.geteuid() == 0, 'Linux root helper')
    def test_usb_disk_does_not_follow_symlinks(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            target = root / 'keep'
            target.write_bytes(b'keep' * 128)
            (root / 'disk.img').symlink_to(target)
            with patch.multiple(media, USB_DIR=root, USB_IMAGE=root / 'disk.img'), \
                    patch.object(media.vim4media, 'DATA_DIR', root):
                with self.assertRaises(OSError):
                    media.usb_disk_fd()
            self.assertEqual(target.read_bytes(), b'keep' * 128)

    def test_descriptor_survives_path_replacement(self):
        with tempfile.TemporaryDirectory() as directory:
            media.IMAGES = Path(directory)
            self.addCleanup(patch.stopall)
            patch.dict(media.vim4media.IMAGE_ROOTS, shared=media.IMAGES).start()
            path = media.IMAGES / 'disk.img'
            path.write_bytes(b'a' * 512)
            fd = media.image_fd(str(path), False)
            try:
                path.unlink()
                path.write_bytes(b'b' * 512)
                self.assertEqual(os.read(fd, 512), b'a' * 512)
            finally:
                os.close(fd)

    def test_invalid_backing_files_are_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            media.IMAGES = Path(directory)
            self.addCleanup(patch.stopall)
            patch.dict(media.vim4media.IMAGE_ROOTS, shared=media.IMAGES).start()
            (media.IMAGES / 'short.img').write_bytes(b'x')
            (media.IMAGES / 'link.img').symlink_to('/etc/passwd')
            os.mkfifo(media.IMAGES / 'pipe.img')
            for name in ('short.img', 'link.img', 'pipe.img', '../outside.img'):
                with self.subTest(name=name), self.assertRaises((OSError, ValueError)):
                    media.image_fd(str(media.IMAGES / name), False)

    def test_eject_clears_backing_file_and_restores_controller(self):
        with tempfile.TemporaryDirectory() as directory:
            media.GADGET = Path(directory)
            media.LUN = media.GADGET / 'lun'
            media.LUN.mkdir()
            (media.GADGET / 'configs/c.1').mkdir(parents=True)
            (media.GADGET / 'UDC').write_text('controller')
            for key, value in {'file': '/old.img', 'cdrom': '1', 'ro': '0'}.items():
                (media.LUN / key).write_text(value)
            media.configure(None, False, True)
            self.assertEqual((media.LUN / 'file').read_text().strip(), '')
            self.assertEqual((media.LUN / 'ro').read_text(), '1')
            self.assertEqual((media.GADGET / 'UDC').read_text(), 'controller')

    def gadget(self, directory, links):
        media.GADGET = Path(directory)
        media.LUN = media.GADGET / 'functions/mass_storage.disk0/lun.0'
        media.LUN.mkdir(parents=True)
        for name in media.FUNCTIONS:
            (media.GADGET / 'functions' / name).mkdir(parents=True, exist_ok=True)
        (media.GADGET / 'configs/c.1').mkdir(parents=True)
        (media.GADGET / 'UDC').write_text('controller')
        for key in ('file', 'cdrom', 'ro'):
            (media.LUN / key).write_text('')
        for name in links:
            (media.GADGET / 'configs/c.1' / name).symlink_to(media.GADGET / 'functions' / name)

    def linked(self):
        return sorted(p.name for p in (media.GADGET / 'configs/c.1').iterdir() if p.is_symlink())

    def test_microphone_replaces_network_and_keeps_media(self):
        with tempfile.TemporaryDirectory() as directory:
            self.gadget(directory, ['ncm.usb0'])
            media.set_microphone(True)
            self.assertEqual(self.linked(), ['uac2.usb0'])
            (media.LUN / 'file').write_text('/proc/self/fd/9')
            media.configure(None, False, True)  # eject keeps the microphone
            self.assertEqual(self.linked(), ['uac2.usb0'])
            media.set_microphone(False)
            self.assertEqual(self.linked(), ['ncm.usb0'])
            self.assertEqual((media.GADGET / 'UDC').read_text(), 'controller')

    def test_missing_microphone_function_leaves_gadget_unchanged(self):
        with tempfile.TemporaryDirectory() as directory:
            self.gadget(directory, ['ncm.usb0'])
            (media.GADGET / 'functions/uac2.usb0').rmdir()
            with self.assertRaises(RuntimeError):
                media.set_microphone(True)
            self.assertEqual(self.linked(), ['ncm.usb0'])


@unittest.skipUnless(os.geteuid() == 0, 'shared lock files require root')
class SharedMediaTest(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        self.root = Path(temp.name)
        roots = {source: self.root / source for source in ('shared', 'jetkvm', 'onekvm', 'pikvm')}
        for root in roots.values():
            root.mkdir()
            (root / 'install.iso').write_bytes(b'x' * 512)
            (root / 'disk.img').write_bytes(b'x' * 512)
        data = self.root / 'data'
        data.mkdir()
        (data / 'disk.img').write_bytes(b'x' * 512)
        (data / 'work.img').write_bytes(b'x' * 512)
        lun = self.root / 'lun'
        lun.write_text('')
        self.addCleanup(patch.stopall)
        patch.multiple(media.vim4media, IMAGE_ROOTS=roots, DATA_DIR=data, LUN_FILE=lun,
                       LOCKS=(self.root / 'lock1', self.root / 'lock2')).start()
        patch.multiple(media, USB_DIR=data, USB_IMAGE=data / 'disk.img', USB_ENABLED=data / 'enabled').start()
        patch.object(media.os, 'sync').start()

    def test_occupied_lun_and_stale_eject_never_reconfigure(self):
        current = str(media.vim4media.DATA_DIR / 'work.img')
        media.vim4media.LUN_FILE.write_text(current)
        for args in (['mount', 'shared/disk.img', 'false', 'true'],
                     ['disk', 'on'], ['disk', 'off'],
                     ['disk', 'off', current], ['eject', str(media.USB_IMAGE)]):
            with self.subTest(args=args), patch.object(media, 'configure') as configure:
                with self.assertRaises(ValueError):
                    media.main(args)
                configure.assert_not_called()
        def eject(fd, cdrom, readonly):
            for lock in media.vim4media.LOCKS:
                with lock.open('r+') as contender:
                    with self.assertRaises(BlockingIOError):
                        fcntl.flock(contender, fcntl.LOCK_EX | fcntl.LOCK_NB)
        with patch.object(media, 'configure', side_effect=eject):
            media.main(['eject', current])

    def test_rollback_data_disk_uses_validated_locked_descriptor(self):
        lun = self.root / 'attributes'
        lun.mkdir()
        disk = media.vim4media.DATA_DIR / 'work.img'
        for key, value in {'file': str(disk), 'cdrom': '0', 'ro': '0'}.items():
            (lun / key).write_text(value)
        def failed_relink(links, prepare, restore):
            prepare()
            restore()
            fd = int((lun / 'file').read_text().split('/')[-1])
            # restore closes its descriptor only after the kernel attach write.
            self.assertGreaterEqual(fd, 0)
            raise OSError('relink failed')
        writes = Path.write_text
        def checked_write(path, value, *args, **kwargs):
            if path == lun / 'file' and value.startswith('/proc/self/fd/'):
                fd = int(value.split('/')[-1])
                self.assertEqual(os.fstat(fd).st_ino, disk.stat().st_ino)
                with disk.open('r+') as contender:
                    with self.assertRaises(BlockingIOError):
                        fcntl.flock(contender, fcntl.LOCK_EX | fcntl.LOCK_NB)
            return writes(path, value, *args, **kwargs)
        with patch.object(media, 'LUN', lun), patch.object(media, 'relink', side_effect=failed_relink), \
                patch.object(Path, 'write_text', checked_write), media.vim4media.media_locks():
            with self.assertRaisesRegex(OSError, 'relink failed'):
                media.configure(None, False, True)

    def test_foreign_mount_modes_and_both_locks_cover_open_and_attach(self):
        def configure(fd, cdrom, readonly):
            self.assertTrue(readonly)
            self.assertEqual(fcntl.fcntl(fd, fcntl.F_GETFL) & os.O_ACCMODE, os.O_RDONLY)
            for lock in media.vim4media.LOCKS:
                with lock.open('r+') as contender:
                    with self.assertRaises(BlockingIOError):
                        fcntl.flock(contender, fcntl.LOCK_EX | fcntl.LOCK_NB)
        with patch.object(media, 'configure', side_effect=configure) as attach:
            media.main(['mount', 'jetkvm/disk.img', 'false', 'false'])
            self.assertFalse(attach.call_args.args[1])
            media.main(['mount', 'pikvm/install.iso', 'false', 'false'])
            self.assertTrue(attach.call_args.args[1])

    def test_data_mount_and_legacy_switch_respect_active_edit(self):
        def configure(fd, cdrom, readonly):
            self.assertFalse(cdrom)
            self.assertFalse(readonly)
            os.write(fd, b'host')
        with patch.object(media, 'configure', side_effect=configure):
            media.main(['mount', 'data/work.img', 'false', 'false'])
            media.main(['disk', 'on'])
        self.assertEqual((media.vim4media.DATA_DIR / 'work.img').read_bytes()[:4], b'host')
        for name, args in [('work.img', ['mount', 'data/work.img', 'false', 'false']),
                           ('disk.img', ['disk', 'on'])]:
            with media.vim4media.edit_disk(name), patch.object(media, 'configure') as attach:
                with self.assertRaisesRegex(ValueError, '编辑'):
                    media.main(args)
                attach.assert_not_called()

    def test_catalog_includes_foreign_sources_and_all_data_disks(self):
        output = io.StringIO()
        with redirect_stdout(output), patch.object(media, 'configure') as attach:
            media.main(['catalog'])
        ids = {entry['id'] for entry in json.loads(output.getvalue())}
        self.assertTrue({'jetkvm/install.iso', 'pikvm/disk.img', 'data/disk.img', 'data/work.img'} <= ids)
        attach.assert_not_called()


if __name__ == '__main__':
    unittest.main()
