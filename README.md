#sz

sz provides a `Reader` and `Writer` for using the [framing format][framing]
for Google's [Snappy][snappy] compression algorithm from Go code.  If you
just want to encode and decode bare blocks, you don't need this; use [the
snappy-go library][snappy-go].

sz includes [a modified version of snappy-go][cport] with the encoding logic
ported to C.  It's compiled into your binary: target systems do not need the
snappy library, though building the package requires a C compiler targeting
the right platform.  The port ran about 50% faster for my compressible
test content.  It passes the original test suite, but that's all the
assurance we have, so be aware.  If you want to revert to the pure-Go
version, just edit the import statement in your copy of [sz.go][szgo].

In general, sz is not a mature, tested implementation. Some things that
would be great:

* Tests. Simple "you can round-trip a file" test, odd read/write/block size
  patterns.
* A "raw" Writer that makes each Write call output a compressed block, rather
  than buffering up 64KB unless flushed/closed earlier.
* Smart skipping of long incompressible runs: after several blocks fail to
  compress, stop trying for a while, or only test-compress small samples.

Randall Farmer, 2014. `sz` is under an MIT license whose full text is in
`LICENSE`.  `sz/snappy` is under a BSD-style license whose full text is in
`snappy/LICENSE`.  There is no endorsement by the Snappy-Go project.

[framing]: https://code.google.com/p/snappy/source/browse/trunk/framing_format.txt
[snappy]: https://code.google.com/p/snappy/
[snappy-go]: https://code.google.com/p/snappy-go/
[cport]: https://github.com/twotwotwo/sz/tree/master/snappy/
[gipfeli]: https://github.com/google/gipfeli/
[szgo]: https://github.com/twotwotwo/sz/tree/master/sz.go
