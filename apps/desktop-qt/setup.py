#!/usr/bin/env python3
"""Setup script for packaging."""

from setuptools import setup, find_packages
from pathlib import Path

readme = Path(__file__).parent / 'README.md'
long_description = readme.read_text() if readme.exists() else ''

setup(
    name='clipshare-desktop',
    version='0.2.0',
    description='ClipShare Desktop Application - Qt6-based clip editor',
    long_description=long_description,
    author='ClipShare',
    python_requires='>=3.10',
    packages=find_packages(where='src'),
    package_dir={'': 'src'},
    install_requires=[
        'PyQt6>=6.7.0',
        'requests>=2.31.0',
        'python-dotenv>=1.0.0',
    ],
    entry_points={
        'console_scripts': [
            'clipshare=main:main',
        ],
    },
    classifiers=[
        'Development Status :: 3 - Alpha',
        'Intended Audience :: End Users/Desktop',
        'License :: OSI Approved :: MIT License',
        'Programming Language :: Python :: 3.10',
        'Programming Language :: Python :: 3.11',
        'Programming Language :: Python :: 3.12',
    ],
)