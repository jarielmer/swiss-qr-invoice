package swissqrinvoice

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"unicode/utf8"
)

// Reference types as defined by the Swiss QR bill standard.
const (
	ReferenceTypeQRR  = "QRR"  // QR reference, requires a QR-IBAN
	ReferenceTypeSCOR = "SCOR" // Creditor Reference (ISO 11649), requires a normal IBAN
	ReferenceTypeNON  = "NON"  // No reference, requires a normal IBAN
)

const maxNameLength = 70

// Validate checks the invoice for combinations the QR bill standard does not
// allow. It is called before rendering.
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

	// Structured address (type S): name, postal code, town and country are mandatory.
	if strings.TrimSpace(i.ReceiverName) == "" {
		return errors.New("creditor name is required")
	}
	if strings.TrimSpace(i.ReceiverZIPCode) == "" {
		return errors.New("creditor postal code is required")
	}
	if strings.TrimSpace(i.ReceiverPlace) == "" {
		return errors.New("creditor town is required")
	}
	if strings.TrimSpace(i.ReceiverCountry) == "" {
		return errors.New("creditor country is required")
	}
	if utf8.RuneCountInString(i.ReceiverName) > maxNameLength {
		return fmt.Errorf("creditor name exceeds %d characters", maxNameLength)
	}
	if utf8.RuneCountInString(i.PayeeName) > maxNameLength {
		return fmt.Errorf("debtor name exceeds %d characters", maxNameLength)
	}
	return nil
}

func normalize(s string) string {
	return strings.ToUpper(strings.ReplaceAll(s, " ", ""))
}

// validIBAN checks a CH/LI IBAN (21 characters) with the ISO 13616 mod 97 checksum.
func validIBAN(iban string) bool {
	if len(iban) != 21 || (!strings.HasPrefix(iban, "CH") && !strings.HasPrefix(iban, "LI")) {
		return false
	}
	return mod97(iban[4:]+iban[:4]) == 1
}

// isQRIBAN reports whether the institution ID (positions 5-9) is in the QR-IID range 30000-31999.
func isQRIBAN(iban string) bool {
	if len(iban) < 9 {
		return false
	}
	iid := iban[4:9]
	return iid >= "30000" && iid <= "31999"
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
	if len(ref) != 27 {
		return false
	}
	table := [10]int{0, 9, 4, 6, 8, 2, 7, 1, 3, 5}
	carry := 0
	for _, r := range ref[:26] {
		if r < '0' || r > '9' {
			return false
		}
		carry = table[(carry+int(r-'0'))%10]
	}
	check := ref[26]
	return check >= '0' && check <= '9' && (10-carry)%10 == int(check-'0')
}

// validCreditorReference checks an ISO 11649 reference (RF + 2 check digits + up to 21 chars).
func validCreditorReference(ref string) bool {
	if len(ref) < 5 || len(ref) > 25 || !strings.HasPrefix(ref, "RF") {
		return false
	}
	return mod97(ref[4:]+ref[:4]) == 1
}
