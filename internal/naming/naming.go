// Package naming holds the rules for the names users give scripts, flows, templates and themes,
// and for the identifiers templates read, such as parameters and step IDs.
package naming

import "regexp"

var validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// IsValid tells whether name is safe as a file or folder name and easy to type in `noct run`.
func IsValid(name string) bool {
	return validName.MatchString(name)
}

// IsIdentifier tells whether name can be read in a template as {{.name}}: letters, digits and _,
// not starting with a digit.
func IsIdentifier(name string) bool {
	return identifier.MatchString(name)
}
