"""Offline release consumer checks; no download, Homebrew or real installation."""
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent.parent


class ReleaseLayout(unittest.TestCase):
    def test_formula_consumer_layout(self):
        for workflow in ('build-release.yml', 'release-please.yml'):
            text = (ROOT / '.github/workflows' / workflow).read_text()
            self.assertIn('tar czf "${ARCHIVE_NAME}" "${BINARY_NAME}" observability assets/pi/rotta-extension.ts', text)
            self.assertIn('bin.install "rotta"', text)
            self.assertIn('(prefix/"rotta-assets").install "observability", "assets"', text)
        # Model the exact archive tree and formula destination next to bin/rotta.
        with tempfile.TemporaryDirectory() as directory:
            prefix = Path(directory)
            (prefix / 'bin').mkdir()
            (prefix / 'bin/rotta').write_bytes(b'binary')
            for source in ('observability/install.py', 'observability/compose.yaml', 'observability/opencode-telemetry.mjs',
                           'assets/pi/rotta-extension.ts'):
                destination = prefix / 'rotta-assets' / source
                destination.parent.mkdir(parents=True, exist_ok=True)
                destination.write_bytes((ROOT / source).read_bytes())
            self.assertEqual((prefix / 'rotta-assets/observability/install.py').resolve().parent.parent,
                             (prefix / 'bin/rotta').resolve().parent.parent / 'rotta-assets')

    def test_shell_reinstall_refuses_foreign_assets_before_binary(self):
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory)
            archive = base / 'release.tar.gz'
            source = base / 'source'
            for name, data in (('rotta', b'new binary'), ('observability/install.py', b'installer'),
                               ('assets/pi/rotta-extension.ts', b'extension')):
                path = source / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(data)
            with tarfile.open(archive, 'w:gz') as bundle:
                for name in ('rotta', 'observability/install.py', 'assets/pi/rotta-extension.ts'):
                    bundle.add(source / name, name)
            fakebin = base / 'fakebin'
            fakebin.mkdir()
            curl = fakebin / 'curl'
            curl.write_text('#!/bin/sh\ncase "$*" in *".sha256"*) exit 1;; esac\nwhile [ "$1" != "-o" ]; do shift; done\ncp "$ARCHIVE_FIXTURE" "$2"\n')
            curl.chmod(0o700)
            target = base / 'bin'
            target.mkdir()
            (target / 'rotta').write_bytes(b'old binary')
            env = dict(os.environ, PATH=str(fakebin) + ':' + os.environ['PATH'],
                       ARCHIVE_FIXTURE=str(archive), ROTTA_VERSION='v0.0.0', ROTTA_INSTALL_DIR=str(target))
            script = ROOT / 'scripts/install-rotta.sh'
            first = subprocess.run(['bash', str(script)], env=env, capture_output=True, text=True)
            self.assertEqual(first.returncode, 0, first.stdout + first.stderr)
            self.assertEqual((target / 'rotta').read_bytes(), b'new binary')
            self.assertEqual((target / 'rotta-assets/observability/install.py').read_bytes(), b'installer')
            (target / 'rotta').write_bytes(b'keep binary')
            (target / 'rotta-assets/observability/install.py').write_bytes(b'foreign')
            second = subprocess.run(['bash', str(script)], env=env, capture_output=True, text=True)
            self.assertNotEqual(second.returncode, 0)
            self.assertEqual((target / 'rotta').read_bytes(), b'keep binary')
            self.assertEqual((target / 'rotta-assets/observability/install.py').read_bytes(), b'foreign')


if __name__ == '__main__':
    unittest.main()
