# Parity fixture generator

Uses [bcdec](https://github.com/iOrange/bcdec) v0.98 at revision
`93628fe5627102fe5187b7eeb99122dec6612c36`.

Run `make gen-parity-fixtures` downloads that exact header into this directory,
verifies its SHA-256, and removes it after generation.

Run `make gen-parity-fixtures` explicitly.
`go test` consumes only committed fixtures and never requires a C compiler.
