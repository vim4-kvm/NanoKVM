import importlib.machinery
import importlib.util
from pathlib import Path
import unittest
from unittest.mock import patch

loader = importlib.machinery.SourceFileLoader('system', str(Path(__file__).with_name('vim4-system')))
spec = importlib.util.spec_from_loader(loader.name, loader)
system = importlib.util.module_from_spec(spec)
loader.exec_module(system)


class SystemTest(unittest.TestCase):
    def test_dns_validation_happens_before_commands(self):
        with patch.object(system, 'run') as command:
            for request in ({'mode': 'manual', 'servers': []},
                            {'mode': 'manual', 'servers': ['1.2.3.4;reboot']},
                            {'mode': 'dhcp', 'servers': ['1.1.1.1']},
                            {'mode': 'invalid'}):
                with self.subTest(request=request), self.assertRaises(ValueError):
                    system.dns('dns-set', request)
            command.assert_not_called()

    def test_dns_failed_reapply_restores_both_families(self):
        old = {'ipv4.dns': '', 'ipv6.dns': '', 'ipv4.ignore-auto-dns': 'no', 'ipv6.ignore-auto-dns': 'no'}
        changes, applies = [], []
        def command(*args, **kwargs):
            if args[1:4] == ('--escape', 'no', '-g'):
                return old[args[4]]
            if args[1:3] == ('connection', 'modify'):
                changes.append(dict(zip(args[4::2], args[5::2])))
            if args[1:3] == ('device', 'reapply'):
                applies.append(True)
                if len(applies) == 1:
                    raise RuntimeError('failed reapply')
            return ''
        with patch.object(system, 'dns_profile', return_value=('eth0', 'ethernet', 'uuid')), patch.object(system, 'run', side_effect=command):
            with self.assertRaises(RuntimeError):
                system.dns('dns-set', {'mode': 'manual', 'servers': ['1.1.1.1', '2606:4700:4700::1111']})
        self.assertEqual(changes[-1], old)
        self.assertEqual(changes[0]['ipv6.dns'], '2606:4700:4700::1111')
        self.assertEqual(len(applies), 2)

    def test_address_validation(self):
        self.assertEqual(system.static_addresses('192.168.50.210/24 192.168.50.1\n# comment'),
                         (['192.168.50.210/24'], '192.168.50.1'))
        for value in ('1.2.3.4', '999.1.2.3/24', '192.168.1.2/24 10.0.0.1', '1.2.3.4/24;reboot', '', None, []):
            with self.subTest(value=value), self.assertRaises(ValueError):
                system.static_addresses(value)

    def test_nmcli_escaping(self):
        self.assertEqual(system.fields(r'*:AP\:Name:12\:34\:56\:78\:90\:AB'),
                         ['*', 'AP:Name', '12:34:56:78:90:AB'])

    def test_failed_reapply_restores_profile(self):
        old = {'ipv4.method': 'auto', 'ipv4.addresses': '', 'ipv4.gateway': '', 'ipv4.dns': ''}
        changes = []
        applies = []
        def command(*args, **kwargs):
            if args[2] == 'GENERAL.CON-UUID':
                return '12345678-1234-1234-1234-123456789012'
            if args[1] == '-g':
                return old[args[2]]
            if args[1:3] == ('connection', 'modify'):
                changes.append(dict(zip(args[4::2], args[5::2])))
            if args[1:3] == ('device', 'reapply'):
                applies.append(True)
                if len(applies) == 1:
                    raise RuntimeError('test reapply failure')
            return ''
        with patch.object(system, 'run', side_effect=command):
            with self.assertRaises(RuntimeError):
                system.static_ip('static-set', {'enabled': True, 'ip': '192.168.50.210/24'})
        self.assertEqual(changes[-1], old)
        self.assertEqual(len(applies), 2)

    def test_assistant_prepare_installs_venv_only_when_missing(self):
        for status, installs in (('install ok installed', False), ('', True)):
            calls = []
            def command(*args, **kwargs):
                calls.append(args)
                return status if args[0] == 'dpkg-query' else ''
            with self.subTest(status=status), patch.object(system, 'run', side_effect=command):
                system.execute('assistant-prepare', {})
            self.assertEqual(any(c[:2] == ('apt-get', 'install') for c in calls), installs)
            self.assertIn(('install', '-d', '-o', 'khadas', '-g', 'khadas', '-m', '0755', system.ASSISTANT_DIR), calls)

    def test_invalid_hostnames_do_not_execute(self):
        with patch.object(system, 'run') as command:
            for value in ('bad name', '-option', '../path', 'x;reboot', None, []):
                with self.assertRaises(ValueError):
                    system.execute('hostname-set', {'hostname': value})
            command.assert_not_called()


if __name__ == '__main__':
    unittest.main()
