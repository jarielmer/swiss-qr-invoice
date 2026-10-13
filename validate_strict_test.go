package swissqrinvoice

import (
	"strings"
	"testing"
)

func TestValidateStrict(t *testing.T) {
	tests := []struct {
		name    string
		base    func() Invoice
		mutate  func(*Invoice)
		wantErr string
	}{
		// Check digits and IID must be numeric.
		{"alphabetic IBAN check digits", validSCORInvoice, func(i *Invoice) { i.ReceiverIBAN = "CHAB00762000000000023" }, "invalid IBAN"},
		{"non-numeric IID", validSCORInvoice, func(i *Invoice) { i.ReceiverIBAN = "CH1030A00123456789012" }, "invalid IBAN"},
		{"alphabetic RF check digits", validSCORInvoice, func(i *Invoice) { i.Reference = "RFAB8" }, "invalid creditor reference"},
		// Debtor: all or nothing (name, postal code, town, country).
		{"empty debtor ok", validQRRInvoice, func(i *Invoice) { *i = withoutDebtor(*i) }, ""},
		{"debtor name only", validQRRInvoice, func(i *Invoice) { n := i.PayeeName; *i = withoutDebtor(*i); i.PayeeName = n }, "debtor postal code is required"},
		{"debtor without country", validQRRInvoice, func(i *Invoice) { completeDebtor(i); i.PayeeCountry = "" }, "debtor country is required"},
		{"complete debtor ok", validQRRInvoice, completeDebtor, ""},
		// Field limits (IG v2.3 4.2.2).
		{"creditor street too long", validQRRInvoice, func(i *Invoice) { i.ReceiverStreet = strings.Repeat("s", 71) }, "creditor street exceeds"},
		{"creditor number too long", validQRRInvoice, func(i *Invoice) { i.ReceiverNumber = strings.Repeat("1", 17) }, "creditor building number exceeds"},
		{"creditor postal code too long", validQRRInvoice, func(i *Invoice) { i.ReceiverZIPCode = strings.Repeat("1", 17) }, "creditor postal code exceeds"},
		{"creditor town too long", validQRRInvoice, func(i *Invoice) { i.ReceiverPlace = strings.Repeat("t", 36) }, "creditor town exceeds"},
		{"creditor country invalid", validQRRInvoice, func(i *Invoice) { i.ReceiverCountry = "CHE" }, "creditor country"},
		{"creditor country lower case", validQRRInvoice, func(i *Invoice) { i.ReceiverCountry = "ch" }, "creditor country"},
		{"debtor town too long", validQRRInvoice, func(i *Invoice) { completeDebtor(i); i.PayeePlace = strings.Repeat("t", 36) }, "debtor town exceeds"},
		{"additional info too long", validQRRInvoice, func(i *Invoice) { i.AdditionalInfo = strings.Repeat("ä", 141) }, "additional information exceeds"},
		{"additional info 140 ok", validQRRInvoice, func(i *Invoice) { i.AdditionalInfo = strings.Repeat("ä", 140) }, ""},
		// Currency and amount.
		{"currency USD", validQRRInvoice, func(i *Invoice) { i.Currency = "USD" }, "currency"},
		{"currency EUR ok", validQRRInvoice, func(i *Invoice) { i.Currency = "EUR" }, ""},
		{"amount empty ok", validQRRInvoice, func(i *Invoice) { i.Amount = "" }, ""},
		{"amount zero", validQRRInvoice, func(i *Invoice) { i.Amount = "0.00" }, "amount"},
		{"amount negative", validQRRInvoice, func(i *Invoice) { i.Amount = "-1.00" }, "amount"},
		{"amount three decimals", validQRRInvoice, func(i *Invoice) { i.Amount = "1.001" }, "amount"},
		{"amount no decimals", validQRRInvoice, func(i *Invoice) { i.Amount = "10" }, "amount"},
		{"amount too large", validQRRInvoice, func(i *Invoice) { i.Amount = "1000000000.00" }, "amount"},
		{"amount max ok", validQRRInvoice, func(i *Invoice) { i.Amount = "999 999 999.99" }, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv := tt.base()
			tt.mutate(&inv)
			err := inv.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("want error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestIsQRIBANNumeric(t *testing.T) {
	if isQRIBAN("CH1030A00123456789012") {
		t.Fatal("non-numeric IID must not be a QR-IID")
	}
}

func completeDebtor(i *Invoice) {
	i.PayeeName = "Pia-Maria Rutschmann-Schnyder"
	i.PayeeStreet = "Grosse Marktgasse"
	i.PayeeNumber = "28"
	i.PayeeZIPCode = "9400"
	i.PayeePlace = "Rorschach"
	i.PayeeCountry = "CH"
}

func withoutDebtor(i Invoice) Invoice {
	i.PayeeName, i.PayeeStreet, i.PayeeNumber = "", "", ""
	i.PayeeZIPCode, i.PayeePlace, i.PayeeCountry = "", "", ""
	return i
}
