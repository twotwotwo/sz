#sz

sz provides a Go `Reader` and `Writer` for the [framing format][framing] for Google's [Snappy][snappy] compression algorithm. If you just want to encode and decode bare blocks, you don't want this; use [the snappy-go library][snappy-go]. It is (so far) far from a mature, well-tested implementation, so use it at your own risk; contributions in the form of tests, bug reports, fixes or improvements are welcome.

Randall Farmer, 2014. MIT licensed; the full text is in LICENSE.

[framing]: https://code.google.com/p/snappy/source/browse/trunk/framing_format.txt
[snappy]: https://code.google.com/p/snappy/
[snappy-go]: https://code.google.com/p/snappy-go/