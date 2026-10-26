package infra

import "testing"

func TestLookupRegion(t *testing.T) {
	cases := []struct {
		provider, region string
		want             residency
		country          string
	}{
		{"aws", "eu-central-1", residencyEEA, "DE"},
		{"aws", "eu-north-1", residencyEEA, "SE"},
		{"aws", "eu-west-2", residencyAdequate, "GB"},
		{"aws", "eu-central-2", residencyAdequate, "CH"},
		{"aws", "us-east-1", residencyThird, ""},
		{"aws", "us-gov-west-1", residencyThird, ""},
		{"aws", "ap-southeast-1", residencyThird, ""},
		{"aws", "ca-central-1", residencyAdequate, "CA"},
		{"aws", "eu-mars-9", residencyUnknown, ""},
		{"google", "EU", residencyEEA, "EU"},
		{"google", "europe-west4", residencyEEA, "NL"},
		{"google", "europe-west4-a", residencyEEA, "NL"},
		{"google", "europe-west2", residencyAdequate, "GB"},
		{"google", "US", residencyThird, "US"},
		{"google", "us-central1", residencyThird, ""},
		{"google", "asia-south1", residencyThird, ""},
		{"google", "global", residencyUnknown, ""},
		{"azurerm", "West Europe", residencyEEA, "NL"},
		{"azurerm", "norwayeast", residencyEEA, "NO"},
		{"azurerm", "UK South", residencyAdequate, "GB"},
		{"azurerm", "East US 2", residencyThird, ""},
		{"azurerm", "southeastasia", residencyThird, ""},
		{"azurerm", "somewhere", residencyUnknown, ""},
	}
	for _, tc := range cases {
		got := lookupRegion(tc.provider, tc.region)
		if got.where != tc.want || got.country != tc.country {
			t.Errorf("%s %s: got %+v, want %d %s", tc.provider, tc.region, got, tc.want, tc.country)
		}
	}
}
