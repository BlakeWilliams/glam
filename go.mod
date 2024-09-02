module github.com/blakewilliams/goat

go 1.23.0

require (
	github.com/stretchr/testify v1.9.0
	golang.org/x/net v0.28.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace golang.org/x/net => github.com/blakewilliams/net-go-fork v0.0.0-20240901202837-19f60ff046bc
