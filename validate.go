package swissqrinvoice

import (
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Reference types as defined by the Swiss QR bill standard.
const (
	ReferenceTypeQRR  = "QRR"  // QR reference, requires a QR-IBAN
	ReferenceTypeSCOR = "SCOR" // Creditor Reference (ISO 11649), requires a normal IBAN
	ReferenceTypeNON  = "NON"  // No reference, requires a normal IBAN
)

// Field length limits (Swiss Implementation Guidelines QR-bill v2.3, 4.2.2).
const (
	maxNameLength           = 70
	maxStreetLength         = 70
	maxBuildingNumberLength = 16
	maxPostalCodeLength     = 16
	maxTownLength           = 35
	maxAdditionalInfoLength = 140
)

var (
	reCountry = regexp.MustCompile(`^[A-Z]{2}$`)
	reAmount  = regexp.MustCompile(`^\d{1,9}\.\d{2}$`)
)

// Validate checks the invoice for combinations and values the QR bill standard
// does not allow. It is called before rendering.
func (i Invoice) Validate() error {
	iban := normalize(i.ReceiverIBAN)
	if !validIBAN(iban) {
		return fmt.Errorf("invalid IBAN %q", i.ReceiverIBAN)
	}
	qrIBAN := isQRIBAN(iban)
	ref := normalize(i.Reference)

	switch i.ReferenceType {
	case ReferenceTypeQRR:
		if !qrIBAN {
			return errors.New("reference type QRR requires a QR-IBAN")
		}
		if !validQRReference(ref) {
			return fmt.Errorf("invalid QR reference %q", i.Reference)
		}
	case ReferenceTypeSCOR:
		if qrIBAN {
			return errors.New("reference type SCOR must not be used with a QR-IBAN")
		}
		if !validCreditorReference(ref) {
			return fmt.Errorf("invalid creditor reference %q", i.Reference)
		}
	case ReferenceTypeNON:
		if qrIBAN {
			return errors.New("reference type NON must not be used with a QR-IBAN")
		}
	default:
		return fmt.Errorf("unknown reference type %q", i.ReferenceType)
	}

	// Creditor: structured address (type S) is mandatory.
	if err := validateAddress("creditor", i.ReceiverName, i.ReceiverStreet, i.ReceiverNumber, i.ReceiverZIPCode, i.ReceiverPlace, i.ReceiverCountry); err != nil {
		return err
	}
	// Debtor: either completely empty or a complete structured address.
	if !i.noDebtorData() {
		if err := validateAddress("debtor", i.PayeeName, i.PayeeStreet, i.PayeeNumber, i.PayeeZIPCode, i.PayeePlace, i.PayeeCountry); err != nil {
			return err
		}
	}

	if utf8.RuneCountInString(i.AdditionalInfo) > maxAdditionalInfoLength {
		return fmt.Errorf("additional information exceeds %d characters", maxAdditionalInfoLength)
	}
	if i.Currency != "CHF" && i.Currency != "EUR" {
		return fmt.Errorf("invalid currency %q (CHF or EUR)", i.Currency)
	}
	return validateAmount(i.Amount)
}

// noDebtorData reports whether no debtor field at all is set.
func (i Invoice) noDebtorData() bool {
	return i.PayeeName == "" && i.PayeeStreet == "" && i.PayeeNumber == "" &&
		i.PayeeZIPCode == "" && i.PayeePlace == "" && i.PayeeCountry == ""
}

func validateAddress(party, name, street, number, postalCode, town, country string) error {
	required := []struct{ label, value string }{
		{"name", name}, {"postal code", postalCode}, {"town", town}, {"country", country},
	}
	for _, f := range required {
		if strings.TrimSpace(f.value) == "" {
			return fmt.Errorf("%s %s is required", party, f.label)
		}
	}
	limits := []struct {
		label string
		value string
		max   int
	}{
		{"name", name, maxNameLength},
		{"street", street, maxStreetLength},
		{"building number", number, maxBuildingNumberLength},
		{"postal code", postalCode, maxPostalCodeLength},
		{"town", town, maxTownLength},
	}
	for _, f := range limits {
		if utf8.RuneCountInString(f.value) > f.max {
			return fmt.Errorf("%s %s exceeds %d characters", party, f.label, f.max)
		}
	}
	if !reCountry.MatchString(country) {
		return fmt.Errorf("%s country %q must be a two-letter ISO code", party, country)
	}
	return nil
}

// validateAmount accepts an empty amount (to be filled in by the payer) or a
// positive amount with exactly two decimals up to 999999999.99. Spaces used as
// thousands separators are ignored.
func validateAmount(amount string) error {
	a := strings.ReplaceAll(amount, " ", "")
	if a == "" {
		return nil
	}
	if !reAmount.MatchString(a) {
		return fmt.Errorf("invalid amount %q", amount)
	}
	if strings.Trim(strings.ReplaceAll(a, ".", ""), "0") == "" {
		return fmt.Errorf("invalid amount %q: must be greater than 0", amount)
	}
	return nil
}

func normalize(s string) string {
	return strings.ToUpper(strings.ReplaceAll(s, " ", ""))
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// validIBAN checks a CH/LI IBAN (21 characters): numeric check digits, numeric
// 5 digit institution ID and the ISO 13616 mod 97 checksum.
func validIBAN(iban string) bool {
	if len(iban) != 21 || (!strings.HasPrefix(iban, "CH") && !strings.HasPrefix(iban, "LI")) {
		return false
	}
	if !isDigits(iban[2:4]) || !isDigits(iban[4:9]) {
		return false
	}
	return mod97(iban[4:]+iban[:4]) == 1
}

// isQRIBAN reports whether the institution ID (positions 5-9) is in the QR-IID range 30000-31999.
func isQRIBAN(iban string) bool {
	if len(iban) < 9 || !isDigits(iban[4:9]) {
		return false
	}
	iid, err := strconv.Atoi(iban[4:9])
	return err == nil && iid >= 30000 && iid <= 31999
}

// mod97 converts letters to numbers (A=10 ... Z=35) and returns the value mod 97.
// It returns -1 for characters outside [0-9A-Z].
func mod97(s string) int {
	var digits strings.Builder
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digits.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			fmt.Fprintf(&digits, "%d", r-'A'+10)
		default:
			return -1
		}
	}
	n, ok := new(big.Int).SetString(digits.String(), 10)
	if !ok {
		return -1
	}
	return int(new(big.Int).Mod(n, big.NewInt(97)).Int64())
}

// validQRReference checks length (27 digits) and the recursive mod 10 check digit.
func validQRReference(ref string) bool {
	if len(ref) != 27 || !isDigits(ref) {
		return false
	}
	table := [10]int{0, 9, 4, 6, 8, 2, 7, 1, 3, 5}
	carry := 0
	for _, r := range ref[:26] {
		carry = table[(carry+int(r-'0'))%10]
	}
	return (10-carry)%10 == int(ref[26]-'0')
}

// validCreditorReference checks an ISO 11649 reference (RF + 2 numeric check
// digits + 1 to 21 alphanumeric characters, mod 97 == 1).
func validCreditorReference(ref string) bool {
	if len(ref) < 5 || len(ref) > 25 || !strings.HasPrefix(ref, "RF") || !isDigits(ref[2:4]) {
		return false
	}
	return mod97(ref[4:]+ref[:4]) == 1
}
