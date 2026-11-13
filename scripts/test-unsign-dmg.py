#!/usr/bin/env python3
"""在临时目录用替身工具验证 DMG 脚本，绝不访问真实 Release。"""
import hashlib
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).with_name('unsign-dmg.sh').resolve()


class UnsignDMGTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='gofinder-dmg-test-')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        (self.root / 'scripts').mkdir()
        shutil.copy2(SCRIPT, self.root / 'scripts/unsign-dmg.sh')
        (self.root / 'mygo.json').write_text('{"version": "0.18.9"}\n')
        self.bin = self.root / 'bin'
        self.bin.mkdir()
        # 使用真实摘要工具，避免把摘要更新测试退化为固定输出比较。
        self.env = dict(os.environ, PATH=str(self.bin) + os.pathsep + os.environ['PATH'],
                        TEST_LOG=str(self.root / 'calls'))
        self.tool('hdiutil', '''
printf 'hdiutil %s\n' "$*" >> "$TEST_LOG"
case "$1" in
 convert)
  case "$2" in *"${FAIL_ARCH:-never-match}"*) exit 9;; esac
  cp "$2" "$6"
  printf unsigned >> "$6"
  ;;
 verify) [ "${FAIL_VERIFY:-0}" != 1 ] ;;
 *) exit 8 ;;
esac
''')
        self.tool('codesign', '''
printf 'codesign %s\n' "$*" >> "$TEST_LOG"
[ "${STILL_SIGNED:-0}" = 1 ]
''')
        self.tool('gh', '''
printf 'gh %s\n' "$*" >> "$TEST_LOG"
case "$2" in
 view) printf '%s\n' "${IS_DRAFT:-true}" ;;
 upload) [ "${FAIL_UPLOAD:-0}" != 1 ] ;;
 *) exit 8 ;;
esac
''')
        for arch in ('arm64', 'amd64'):
            self.dmg(arch).parent.mkdir(parents=True)
            self.dmg(arch).write_bytes(arch.encode())
        self.other = 'a' * 64 + '  unrelated.tar.gz\n'
        self.sums = self.root / 'build/SHA256SUMS'
        self.sums.write_text(self.other + 'old  gofinder-0.18.9-arm64.dmg\n' * 2)

    def tool(self, name, body):
        path = self.bin / name
        path.write_text('#!/bin/sh\nset -eu\n' + body)
        path.chmod(0o755)

    def dmg(self, arch):
        return self.root / f'build/darwin-{arch}/gofinder-0.18.9-{arch}.dmg'

    def run_script(self, *args, ok=True, **env):
        result = subprocess.run(['sh', 'scripts/unsign-dmg.sh', *args], cwd=self.root,
                                env=dict(self.env, **env), capture_output=True, text=True)
        self.assertEqual(result.returncode == 0, ok, result.stdout + result.stderr)
        return result

    def log(self):
        path = self.root / 'calls'
        return path.read_text().splitlines() if path.exists() else []

    def assert_sums(self, arches):
        lines = self.sums.read_text().splitlines()
        self.assertIn(self.other.strip(), lines)
        for arch in arches:
            name = self.dmg(arch).name
            matches = [line for line in lines if line.split()[1] == name]
            self.assertEqual(matches, [hashlib.sha256(self.dmg(arch).read_bytes()).hexdigest()
                                       + '  ' + name])

    def test_default_and_single_arch(self):
        self.run_script()
        self.assertEqual(self.dmg('amd64').read_bytes(), b'amd64')
        self.assert_sums(['arm64'])
        self.run_script('--arch', 'amd64')
        self.assert_sums(['arm64', 'amd64'])

    def test_all_repeat_and_upload_order(self):
        for _ in range(2):
            self.run_script('--upload', '--arch', 'all')
            self.assert_sums(['arm64', 'amd64'])
        lines = self.log()
        uploads = [i for i, line in enumerate(lines) if line.startswith('gh release upload')]
        self.assertEqual(len(uploads), 6)
        self.assertIn('arm64', lines[uploads[0]])
        self.assertIn('amd64', lines[uploads[1]])
        self.assertIn('build/SHA256SUMS', lines[uploads[2]])
        verifies = [i for i, line in enumerate(lines) if line.startswith('hdiutil verify')]
        self.assertLess(verifies[1], uploads[0])
        self.assertLess(verifies[3], uploads[3])

    def test_legacy_upload(self):
        self.run_script('--upload')
        uploads = [line for line in self.log() if line.startswith('gh release upload')]
        self.assertEqual(len(uploads), 2)
        self.assertEqual(self.dmg('amd64').read_bytes(), b'amd64')

    def test_invalid_arguments(self):
        for args in [('--wat',), ('--arch',), ('--arch', 'x86_64')]:
            with self.subTest(args=args):
                self.run_script(*args, ok=False)
        self.assertEqual(self.log(), [])

    def test_missing_input_does_not_convert_or_upload(self):
        self.dmg('amd64').unlink()
        self.run_script('--arch', 'all', '--upload', ok=False)
        self.assertEqual(self.log(), [])

    def test_conversion_or_verification_failure_keeps_inputs(self):
        original = self.sums.read_bytes()
        for env in ({'FAIL_ARCH': 'amd64'}, {'FAIL_VERIFY': '1'}, {'STILL_SIGNED': '1'}):
            with self.subTest(env=env):
                self.run_script('--arch', 'all', '--upload', ok=False, **env)
                self.assertEqual(self.dmg('arm64').read_bytes(), b'arm64')
                self.assertEqual(self.dmg('amd64').read_bytes(), b'amd64')
                self.assertEqual(self.sums.read_bytes(), original)
        self.assertFalse(any(line.startswith('gh release upload') for line in self.log()))

    def test_public_release_is_refused(self):
        self.run_script('--arch', 'all', '--upload', ok=False, IS_DRAFT='false')
        self.assertFalse(any(line.startswith('hdiutil') for line in self.log()))
        self.assertFalse(any(line.startswith('gh release upload') for line in self.log()))

    def test_upload_failure_stops_before_summary(self):
        result = self.run_script('--arch', 'all', '--upload', ok=False, FAIL_UPLOAD='1')
        uploads = [line for line in self.log() if line.startswith('gh release upload')]
        self.assertEqual(len(uploads), 1)
        self.assertNotIn('已替换', result.stdout)

    def test_missing_checksum_file_is_created(self):
        self.sums.unlink()
        self.run_script('--arch', 'all')
        self.assertEqual(len(self.sums.read_text().splitlines()), 2)


if __name__ == '__main__':
    unittest.main()
