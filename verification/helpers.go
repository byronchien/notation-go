package verification

import (
	"fmt"
	"regexp"
	"strings"

	ldapv3 "github.com/go-ldap/ldap/v3"
)

// isPresent is a utility function to check if a string exists in an array
func isPresent(val string, values []string) bool {
	for _, v := range values {
		if v == val {
			return true
		}
	}
	return false
}

// validateRegistryScopeFormat validates if a scope is following the format defined in distribution spec
func validateRegistryScopeFormat(scope string) error {
	// Domain and Repository regexes are adapted from distribution implementation
	// https://github.com/distribution/distribution/blob/main/reference/regexp.go#L31
	domainRegexp := regexp.MustCompile(`^(?:[a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9])(?:(?:\.(?:[a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9]))+)?(?::[0-9]+)?$`)
	repositoryRegexp := regexp.MustCompile(`^[a-z0-9]+(?:(?:(?:[._]|__|[-]*)[a-z0-9]+)+)?(?:(?:/[a-z0-9]+(?:(?:(?:[._]|__|[-]*)[a-z0-9]+)+)?)+)?$`)
	errorMessage := "registry scope %q is not valid, make sure it is the fully qualified registry URL without the scheme/protocol. e.g domain.com/my/repository"
	firstSlash := strings.Index(scope, "/")
	if firstSlash < 0 {
		return fmt.Errorf(errorMessage, scope)
	}
	domain := scope[:firstSlash]
	repository := scope[firstSlash+1:]

	if domain == "" || repository == "" || !domainRegexp.MatchString(domain) || !repositoryRegexp.MatchString(repository) {
		return fmt.Errorf(errorMessage, scope)
	}

	// No errors
	return nil
}

// validateDistinguishedName validates if a DN name is parsable and follows Notary V2 rules
func validateDistinguishedName(name string) (map[string]string, error) {
	mandatoryFields := []string{"C", "ST", "O"}
	attrKeyValue := make(map[string]string)
	dn, err := ldapv3.ParseDN(name)

	if err != nil {
		return nil, fmt.Errorf("distinguished name (DN) %q is not valid, it must contain 'C', 'ST', and 'O' RDN attributes at a minimum, and follow RFC 4514 standard", name)
	}

	for _, rdn := range dn.RDNs {

		// multi-valued RDNs are not supported (TODO: add spec reference here)
		if len(rdn.Attributes) > 1 {
			return nil, fmt.Errorf("distinguished name (DN) %q has multi-valued RDN attributes, remove multi-valued RDN attributes as they are not supported", name)
		}
		for _, attribute := range rdn.Attributes {
			if attrKeyValue[attribute.Type] == "" {
				attrKeyValue[attribute.Type] = attribute.Value
			} else {
				return nil, fmt.Errorf("distinguished name (DN) %q has duplicate RDN attribute for %q, DN can only have unique RDN attributes", name, attribute.Type)
			}
		}
	}

	// Verify mandatory fields are present
	for _, field := range mandatoryFields {
		if attrKeyValue[field] == "" {
			return nil, fmt.Errorf("distinguished name (DN) %q has no mandatory RDN attribute for %q, it must contain 'C', 'ST', and 'O' RDN attributes at a minimum", name, field)
		}
	}
	// No errors
	return attrKeyValue, nil
}

func validateOverlappingDNs(policyName string, parsedDNs []parsedDN) error {
	for i, dn1 := range parsedDNs {
		for j, dn2 := range parsedDNs {
			if i != j && isOverlappingDN(dn1.ParsedMap, dn2.ParsedMap) {
				return fmt.Errorf("trust policy statement %q has overlapping x509 trustedIdentities, %q overlaps with %q", policyName, dn1.RawString, dn2.RawString)
			}
		}
	}

	return nil
}

func isOverlappingDN(dn1 map[string]string, dn2 map[string]string) bool {
	for key := range dn1 {
		if dn1[key] != dn2[key] {
			return false
		}
	}
	return true
}