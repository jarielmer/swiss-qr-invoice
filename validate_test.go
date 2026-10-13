package swissqrinvoice

import (
	"strings"
	"testing"
)

func validQRRInvoice() Invoice {
	return Invoice{
		ReceiverIBAN:    "CH44 3199 9123 0008 8901 2",
		ReferenceType:   ReferenceTypeQRR,
		ReceiverName:    "Robert Schneider AG",
		ReceiverStreet:  "Rue du Lac",
		ReceiverNumber:  "12",
		ReceiverZIPCode: "2501",
		ReceiverPlace:   "Biel",
		ReceiverCountry: "CH",
		PayeeName:       "Pia-Maria Rutschmann-Schnyder",
		PayeeStreet:     "Grosse Marktgasse",
		PayeeNumber:     "28",
		PayeeZIPCode:    "9400",
		PayeePlace:      "Rorschach",
		PayeeCountry:    "CH",
		Reference:       "21 00000 00003 13947 14300 09017",
		Amount:          "3 949.75",
		Currency:        "CHF",
	}
}

func validSCORInvoice() Invoice {
	i := validQRRInvoice()
	i.ReceiverIBAN = "CH93 0076 2011 6238 5295 7"
	i.ReferenceType = ReferenceTypeSCOR
	i.Reference = "RF18 5390 0754 7034"
	return i
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		base    func() Invoice
		mutate  func(*Invoice)
		wantErr string
	}{
		{"valid QRR", validQRRInvoice, func(i *Invoice) {}, ""},
		{"valid SCOR", validSCORInvoice, func(i *Invoice) {}, ""},
		{"valid NON", validSCORInvoice, func(i *Invoice) { i.ReferenceType = ReferenceTypeNON; i.Reference = "" }, ""},
		{"LI IBAN", validSCORInvoice, func(i *Invoice) { i.ReceiverIBAN = "LI21 0881 0000 2324 013A A" }, ""},
		{"bad IBAN checksum", validSCORInvoice, func(i *Invoice) { i.ReceiverIBAN = "CH94 0076 2011 6238 5295 7" }, "invalid IBAN"},
		{"foreign IBAN", validSCORInvoice, func(i *Invoice) { i.ReceiverIBAN = "DE89370400440532013000" }, "invalid IBAN"},
		{"QRR with normal IBAN", validQRRInvoice, func(i *Invoice) { i.ReceiverIBAN = "CH93 0076 2011 6238 5295 7" }, "requires a QR-IBAN"},
		{"SCOR with QR-IBAN", validSCORInvoice, func(i *Invoice) { i.ReceiverIBAN = "CH44 3199 9123 0008 8901 2" }, "must not be used with a QR-IBAN"},
		{"NON with QR-IBAN", validQRRInvoice, func(i *Invoice) { i.ReferenceType = ReferenceTypeNON; i.Reference = "" }, "must not be used with a QR-IBAN"},
		{"QRR bad check digit", validQRRInvoice, func(i *Invoice) { i.Reference = "21 00000 00003 13947 14300 09018" }, "invalid QR reference"},
		{"QRR wrong length", validQRRInvoice, func(i *Invoice) { i.Reference = "2100000000031394714300090" }, "invalid QR reference"},
		{"SCOR bad checksum", validSCORInvoice, func(i *Invoice) { i.Reference = "RF19 5390 0754 7034" }, "invalid creditor reference"},
		{"SCOR too long", validSCORInvoice, func(i *Invoice) { i.Reference = "RF00" + strings.Repeat("1", 22) }, "invalid creditor reference"},
		{"unknown type", validQRRInvoice, func(i *Invoice) { i.ReferenceType = "" }, "unknown reference type"},
		{"missing creditor name", validQRRInvoice, func(i *Invoice) { i.ReceiverName = "" }, "creditor name is required"},
		{"missing creditor zip", validQRRInvoice, func(i *Invoice) { i.ReceiverZIPCode = "" }, "postal code is required"},
		{"missing creditor town", validQRRInvoice, func(i *Invoice) { i.ReceiverPlace = "" }, "town is required"},
		{"missing creditor country", validQRRInvoice, func(i *Invoice) { i.ReceiverCountry = "" }, "country is required"},
		{"creditor name too long", validQRRInvoice, func(i *Invoice) { i.ReceiverName = strings.Repeat("a", 71) }, "creditor name exceeds"},
		{"debtor name too long", validQRRInvoice, func(i *Invoice) { i.PayeeName = strings.Repeat("ä", 71) }, "debtor name exceeds"},
		{"name 70 chars ok", validQRRInvoice, func(i *Invoice) { i.PayeeName = strings.Repeat("ä", 70) }, ""},
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

func TestQRContentReferenceType(t *testing.T) {
	non := validSCORInvoice()
	non.ReferenceType = ReferenceTypeNON
	non.Reference = "RF18539007547034"

	cases := []struct {
		inv     Invoice
		refType string
		ref     string
	}{
		{validQRRInvoice(), "QRR", "210000000003139471430009017"},
		{validSCORInvoice(), "SCOR", "RF18539007547034"},
		{non, "NON", ""},
	}
	for _, c := range cases {
		content, err := c.inv.qrContent()
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(content, "\r\n")
		// Reference type and reference are the 28th and 29th elements.
		if lines[27] != c.refType || lines[28] != c.ref {
			t.Fatalf("got type %q ref %q, want %q %q", lines[27], lines[28], c.refType, c.ref)
		}
	}
}

func TestDocRejectsInvalid(t *testing.T) {
	inv := validQRRInvoice()
	inv.ReceiverIBAN = "CH93 0076 2011 6238 5295 7"
	if _, err := inv.Doc(); err == nil {
		t.Fatal("expected Doc() to fail validation")
	}
}

func TestDocValid(t *testing.T) {
	for _, inv := range []Invoice{validQRRInvoice(), validSCORInvoice()} {
		if _, err := inv.Doc(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}
