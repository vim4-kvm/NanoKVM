import importlib.machinery
import importlib.util
from pathlib import Path
import subprocess
import unittest
from unittest.mock import patch

loader = importlib.machinery.SourceFileLoader('update', str(Path(__file__).with_name('vim4-update')))
spec = importlib.util.spec_from_loader(loader.name, loader)
update = importlib.util.module_from_spec(spec)
loader.exec_module(update)


class UpdateTest(unittest.TestCase):
    def test_application_only_upgrade(self):
        with patch.object(update.subprocess, 'check_output', return_value='Inst vim4-nanokvm [1] (2 local-deb [arm64])\nConf vim4-nanokvm (2)\n'):
            update.preflight(Path('/tmp/app.deb'))

    def test_missing_or_outdated_platform_rejected_before_install(self):
        for plan in ('Inst vim4-kvm-board [1] (2)\n', 'Inst vim4-amlvenc (2)\n', 'Remv libopus0 [1]\n'):
            with self.subTest(plan=plan), patch.object(update.subprocess, 'check_output', return_value=plan):
                with self.assertRaisesRegex(ValueError, 'platform packages'):
                    update.preflight(Path('/tmp/app.deb'))

    def test_unsatisfied_dependencies_propagate(self):
        with patch.object(update.subprocess, 'check_output', side_effect=subprocess.CalledProcessError(100, 'apt-get')):
            with self.assertRaises(subprocess.CalledProcessError):
                update.preflight(Path('/tmp/app.deb'))

    def test_restart_only_after_verified_durable_install(self):
        events = []
        with patch.object(update, 'preflight', side_effect=lambda p: events.append('preflight')), \
             patch.object(update, 'verify_installed', side_effect=lambda p: events.append('verify')), \
             patch.object(update.os, 'sync', create=True, side_effect=lambda: events.append('sync')), \
             patch.object(update.subprocess, 'run', side_effect=lambda args, **kw: events.append(args[0])):
            update.apply_update(Path('/tmp/app.deb'), Path('/tmp/root'))
        self.assertEqual(events, ['preflight', 'dpkg', 'verify', 'sync', 'systemd-run'])

    def test_failed_verification_never_schedules_restart(self):
        with patch.object(update, 'preflight'), \
             patch.object(update, 'verify_installed', side_effect=ValueError('damaged helper')), \
             patch.object(update.subprocess, 'run') as run:
            with self.assertRaisesRegex(ValueError, 'damaged helper'):
                update.apply_update(Path('/tmp/app.deb'), Path('/tmp/root'))
        self.assertEqual([call.args[0][0] for call in run.call_args_list], ['dpkg'])


if __name__ == '__main__':
    unittest.main()
