#!/usr/bin/env python3

from setuptools import setup

setup(name='kailing_token',
      version='0.0.1',
      description='kailing token plugin for websockify',
      author='zhangyuyun',
      author_email='yuyunz@kailing.cn',
      url='https://github.com/zhangyyun/webssh',
      packages=["kailing_token"],
      classifiers=[
          "Operating System :: POSIX :: Linux",
          ],
      platforms=[
          "linux_x86_64",
          ],
      package_data={
        'kailing_token': ['get-ip.so'],
      },
      options={"bdist_wheel": {"plat_name": "linux-x86_64"}},
     )

