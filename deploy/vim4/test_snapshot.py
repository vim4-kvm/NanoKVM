"""The assistant must authenticate local capture and reject non-image replies."""
import importlib.util
import os
from pathlib import Path
import sys
import unittest
from unittest.mock import Mock, patch


class SnapshotTests(unittest.TestCase):
    def test_authenticated_capture_does_not_follow_redirects(self):
        response = Mock(status_code=200, headers={'Content-Type': 'image/jpeg'}, content=b'jpeg')
        client = Mock()
        client.get.return_value = response
        spec = importlib.util.spec_from_file_location(
            'assistant_snapshot', Path(__file__).parents[2] / 'support/assistant/snapshot.py')
        module = importlib.util.module_from_spec(spec)
        with patch.dict(sys.modules, requests=client), patch.dict(os.environ, KVM_SNAPSHOT_TOKEN='private-test-token'):
            spec.loader.exec_module(module)
            self.assertEqual(module.fetch_snapshot('http://127.0.0.1:8080/api/stream/snapshot'), b'jpeg')
            options = client.get.call_args.kwargs
            self.assertEqual(options['headers']['X-NanoKVM-Internal-Token'], 'private-test-token')
            self.assertFalse(options['allow_redirects'])
            response.headers = {'Content-Type': 'application/json'}
            with self.assertRaises(ValueError):
                module.fetch_snapshot('http://127.0.0.1:8080/api/stream/snapshot')
            response.raise_for_status.side_effect = RuntimeError('unauthorized')
            with self.assertRaisesRegex(RuntimeError, 'unauthorized'):
                module.fetch_snapshot('http://127.0.0.1:8080/api/stream/snapshot')
