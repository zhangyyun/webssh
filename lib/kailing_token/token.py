#!/usr/bin/env python3

import os
from ctypes import CDLL, c_char_p, POINTER, c_char, cast

class Token:
    def __init__(self, src):
        self.query = None
        self.release = None
        pass

    def lookup(self, token):
        if self.query is None:
            so = CDLL(os.path.dirname(os.path.abspath(__file__)) + "/get-ip.so")
            self.query = so.query
            self.query.argtypes = [c_char_p]
            self.query.restype = POINTER(c_char)
            self.release = so.release
            self.release.argtypes = [c_char_p]

        ip = self.query(token.encode())
        if not ip:
            return None

        ipstr = cast(ip, c_char_p).value.decode()
        self.release(ip)

        return [ipstr, '5901']

if __name__ == "__main__":
    token = Token("")

    print(token.lookup("7ff149ff-b71e-4e11-8330-4f8ea7cb7131"))
