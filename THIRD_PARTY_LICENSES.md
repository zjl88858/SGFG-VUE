# Third-party notices

SGFG v12 uses oh-my-hysteria core/extras and its local QUIC fork at commit
`22b2f02784138663c13e1a0e9e0a06c5dc202dbf`:
https://github.com/zjl88858/oh-my-hysteria

Retain the complete original notices in `third_party/oh-my-hysteria/LICENSE.md`,
`LICENSE-THIRD-PARTY.md`, `third_party/quic-go/LICENSE`,
`third_party/tquic/LICENSE`, and `third_party/geodesic/LICENSE-kcc` and
`LICENSE-ucp`. Paths in this paragraph are relative to the OMHY snapshot.

OMHY original additions use MIT-0. Inherited Hysteria2 and QUIC portions retain
MIT; the TQUIC COPA adaptation retains Apache-2.0; Geodesic adaptations retain
the elected KCC BSD license and UCP MIT. Selecting only COPA in the application
does not remove the upstream code's license obligations. No Rust controller or
kernel congestion module is built. The native Windows build copies all of
these notices into the executable distribution's `licenses/` directory.

## GOST (local SS relay only)

https://github.com/go-gost/x — v0.5.5

MIT License

Copyright (c) 2016 ginuerzh

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

## Other dependencies

Wails, Vue, Go modules and frontend packages retain their respective licenses;
consult `go.mod`, `go.sum`, `frontend/package.json` and dependency distributions.
This document is not a complete transitive dependency license inventory.

NetFilter SDK is separately licensed commercial software. Its runtime/driver
distribution rights are not granted by SGFG or OMHY licenses; retain the vendor
license and obtain the appropriate license/signatures before distribution.
