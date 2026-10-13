import importlib.machinery
import importlib.util
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

loader = importlib.machinery.SourceFileLoader('tailscale', str(Path(__file__).with_name('vim4-tailscale')))
spec = importlib.util.spec_from_loader(loader.name, loader)
tailscale = importlib.util.module_from_spec(spec)
loader.exec_module(tailscale)


class TailscaleTest(unittest.TestCase):
    def test_invalid_actions_do_not_run(self):
        with patch.object(tailscale, 'run') as run:
            for args in ([], ['install', 'x'], ['--help'], ['up; reboot']):
                with self.subTest(args=args), self.assertRaises(ValueError):
                    tailscale.main(args)
        run.assert_not_called()

    def test_operations_require_installation(self):
        with patch.object(tailscale, 'installed', return_value=False), patch.object(tailscale, 'run') as run:
            with self.assertRaises(RuntimeError):
                tailscale.main(['up'])
        run.assert_not_called()

    def test_status_tolerates_cli_preamble(self):
        output = 'Warning: client version differs\n{"BackendState":"Running","Self":{"HostName":"vim4",' \
                 '"TailscaleIPs":["100.64.0.7"]},"CurrentTailnet":{"Name":"example.ts.net"}}'
        result = unittest.mock.Mock(returncode=0, stdout=output)
        with patch.object(tailscale, 'installed', return_value=True), patch.object(tailscale, 'run', return_value=result):
            self.assertEqual(tailscale.status(), {'installed': True, 'running': True, 'backendState': 'Running',
                                                  'hostName': 'vim4', 'ips': ['100.64.0.7'],
                                                  'tailnet': 'example.ts.net'})

    def test_login_returns_url_from_detached_cli(self):
        with tempfile.TemporaryDirectory() as directory:
            log = Path(directory) / 'login.log'

            def start(args, stdout, **kwargs):
                stdout.write('\nTo authenticate, visit:\n\n\thttps://login.tailscale.com/a/abc123\n\n')
                stdout.flush()

            with patch.object(tailscale, 'LOGIN_LOG', log), \
                    patch.object(tailscale, 'status', return_value={'installed': True, 'running': True,
                                                                    'backendState': 'NeedsLogin'}), \
                    patch.object(tailscale.subprocess, 'Popen', side_effect=start) as popen:
                self.assertEqual(tailscale.login(), {'url': 'https://login.tailscale.com/a/abc123'})
            self.assertTrue(popen.call_args.kwargs['start_new_session'])


if __name__ == '__main__':
    unittest.main()
