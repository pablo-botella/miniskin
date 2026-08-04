module github.com/pablo-botella/miniskin

go 1.26

retract [v0.1.0, v0.5.1] // published prematurely; superseded by the rebuilt module
require (
	github.com/pablo-botella/cargoxml v0.5.2
	github.com/pablo-botella/mkskill v0.5.5
	github.com/pablo-botella/mskblob v0.5.1
	github.com/tdewolff/minify/v2 v2.24.13
)

require (
	github.com/pablo-botella/fmlines v0.5.1 // indirect
	github.com/pablo-botella/linereader v0.5.1 // indirect
	github.com/tdewolff/parse/v2 v2.8.12 // indirect
)
