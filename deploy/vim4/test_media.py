import importlib.machinery
import importlib.util
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch


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
            with patch.multiple(media, USB_DIR=root, USB_IMAGE=root / 'disk.img', USB_SIZE=1024 * 1024):
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

    @unittest.skipUnless(hasattr(os, 'posix_fallocate') and os.geteuid() == 0, 'Linux root helper')
    def test_usb_disk_does_not_follow_symlinks(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            target = root / 'keep'
            target.write_bytes(b'keep' * 128)
            (root / 'disk.img').symlink_to(target)
            with patch.multiple(media, USB_DIR=root, USB_IMAGE=root / 'disk.img'):
                with self.assertRaises(OSError):
                    media.usb_disk_fd()
            self.assertEqual(target.read_bytes(), b'keep' * 128)

    def test_descriptor_survives_path_replacement(self):
        with tempfile.TemporaryDirectory() as directory:
            media.IMAGES = Path(directory)
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



if __name__ == '__main__':
    unittest.main()
