#!/usr/bin/python3
import os
from pathlib import Path
import sys
import tempfile


def main():
    # ssh runs this forced command; the deployment key cannot run arbitrary commands.
    root = Path('/srv/mythra/releases')
    maximum = 100 * 1024 * 1024
    descriptor, name = tempfile.mkstemp(prefix='.upload-', dir=root)
    try:
        with os.fdopen(descriptor, 'wb') as output:
            header = sys.stdin.buffer.read(64)
            if len(header) != 64 or header[:2] != b'MZ':
                raise ValueError('expected a windows executable')
            output.write(header)
            size = len(header)
            while chunk := sys.stdin.buffer.read(1024 * 1024):
                size += len(chunk)
                if size > maximum:
                    raise ValueError('executable exceeds size limit')
                output.write(chunk)
            output.flush()
            os.fsync(output.fileno())
        with open(name, 'rb') as binary:
            binary.seek(int.from_bytes(header[60:64], 'little'))
            if binary.read(4) != b'PE\0\0':
                raise ValueError('invalid pe signature')
        os.chmod(name, 0o644)
        os.replace(name, root / 'Mythra.exe')
        print('published Mythra.exe')
    finally:
        Path(name).unlink(missing_ok=True)


if __name__ == '__main__':
    main()
