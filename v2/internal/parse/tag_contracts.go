package parse

// ContractSpecs splits a `contract:` directive into individual contract
// specifications, e.g. "mutex(source),conflicts(a,b)". Tokenization rules match
// validators: split on top-level commas only, respecting parentheses, so a
// contract's own argument list (e.g. conflicts(a,b)) stays intact.
func ContractSpecs(input string) []string {
	return ValidatorSpecs(input)
}
