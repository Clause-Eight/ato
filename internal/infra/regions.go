package infra

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Clause-Eight/pii-eu/internal/domain"
)

// residency is where a region keeps data, as far as the EU transfer rules are concerned.
type residency int

const (
	residencyUnknown residency = iota
	residencyEEA
	// residencyAdequate is outside the EEA, in a country with an EU adequacy decision (GDPR Art. 45).
	residencyAdequate
	residencyThird
)

// regionInfo is a region's country (ISO 3166-1 alpha-2) and residency.
type regionInfo struct {
	country string
	where   residency
}

func eea(cc string) regionInfo      { return regionInfo{cc, residencyEEA} }
func adequate(cc string) regionInfo { return regionInfo{cc, residencyAdequate} }

// Region tables, from each provider's public list of regions. A region that is in none of them and matches no
// third-country prefix is unknown, never assumed to be in or out of the EEA. The adequacy list follows the
// European Commission's adequacy decisions; the United States is treated as a third country because the
// EU-US Data Privacy Framework covers only certified organisations.
var regionTables = map[string]map[string]regionInfo{
	"aws": {
		"eu-central-1": eea("DE"), "eu-west-1": eea("IE"), "eu-west-3": eea("FR"), "eu-south-1": eea("IT"),
		"eu-south-2": eea("ES"), "eu-north-1": eea("SE"), "eusc-de-east-1": eea("DE"),
		"eu-central-2": adequate("CH"), "eu-west-2": adequate("GB"), "ca-central-1": adequate("CA"),
		"ca-west-1": adequate("CA"), "ap-northeast-1": adequate("JP"), "ap-northeast-3": adequate("JP"),
		"ap-northeast-2": adequate("KR"), "il-central-1": adequate("IL"), "ap-southeast-6": adequate("NZ"),
	},
	"google": {
		"eu": eea("EU"), "europe": eea("EU"), "eur4": eea("EU"),
		"europe-west1": eea("BE"), "europe-west3": eea("DE"), "europe-west4": eea("NL"), "europe-west8": eea("IT"),
		"europe-west9": eea("FR"), "europe-west10": eea("DE"), "europe-west12": eea("IT"), "europe-central2": eea("PL"),
		"europe-north1": eea("FI"), "europe-north2": eea("SE"), "europe-southwest1": eea("ES"),
		"europe-west2": adequate("GB"), "europe-west6": adequate("CH"), "me-west1": adequate("IL"),
		"northamerica-northeast1": adequate("CA"), "northamerica-northeast2": adequate("CA"),
		"asia-northeast1": adequate("JP"), "asia-northeast2": adequate("JP"), "asia-northeast3": adequate("KR"),
		"us": {"US", residencyThird}, "asia": {"", residencyThird}, "nam4": {"US", residencyThird}, "asia1": adequate("JP"),
	},
	"azurerm": {
		"westeurope": eea("NL"), "northeurope": eea("IE"), "francecentral": eea("FR"), "francesouth": eea("FR"),
		"germanywestcentral": eea("DE"), "germanynorth": eea("DE"), "swedencentral": eea("SE"), "swedensouth": eea("SE"),
		"norwayeast": eea("NO"), "norwaywest": eea("NO"), "polandcentral": eea("PL"), "italynorth": eea("IT"),
		"spaincentral": eea("ES"), "austriaeast": eea("AT"), "belgiumcentral": eea("BE"), "denmarkeast": eea("DK"),
		"uksouth": adequate("GB"), "ukwest": adequate("GB"), "switzerlandnorth": adequate("CH"), "switzerlandwest": adequate("CH"),
		"canadacentral": adequate("CA"), "canadaeast": adequate("CA"), "japaneast": adequate("JP"), "japanwest": adequate("JP"),
		"koreacentral": adequate("KR"), "koreasouth": adequate("KR"), "israelcentral": adequate("IL"), "newzealandnorth": adequate("NZ"),
	},
}

// thirdCountry matches regions outside the EEA and without an adequacy decision that are not listed one by one.
var thirdCountry = map[string]*regexp.Regexp{
	"aws":    regexp.MustCompile(`^(us|ap|sa|me|af|mx|cn)(-[a-z]+)+-\d+$`),
	"google": regexp.MustCompile(`^(us|asia|australia|southamerica|northamerica|me|africa)-[a-z]+\d+$`),
	"azurerm": regexp.MustCompile(`^((east|west|central|northcentral|southcentral|westcentral)us\d?|brazil(south|southeast)|australia(east|southeast|central|central2)|` +
		`(east|southeast)asia|(central|south|west)india|jioindia(west|central)|southafrica(north|west)|uae(north|central)|qatarcentral|mexicocentral|chilecentral|indonesiacentral|malaysiawest|taiwannorth)$`),
}

var googleZone = regexp.MustCompile(`^([a-z]+-[a-z]+\d+)-[a-z]$`)

// lookupRegion classifies one region of a provider.
func lookupRegion(provider, region string) regionInfo {
	r := strings.ToLower(strings.TrimSpace(region))
	if provider == "azurerm" {
		r = strings.ReplaceAll(r, " ", "")
	}
	if provider == "google" {
		if m := googleZone.FindStringSubmatch(r); m != nil {
			r = m[1]
		}
	}
	if info, ok := regionTables[provider][r]; ok {
		return info
	}
	if re := thirdCountry[provider]; re != nil && re.MatchString(r) {
		return regionInfo{where: residencyThird}
	}
	return regionInfo{}
}

// residencyOf turns a store's regions into its InEEA setting and, for regions outside the EEA that all have an
// adequacy decision, the country of the first one.
func residencyOf(provider string, regions []string, found domain.Setting) (domain.Setting, string) {
	if found.State == domain.Unknown || len(regions) == 0 {
		if found.Why == "" {
			found.Why = "the region cannot be read from the code"
		}
		return domain.Setting{State: domain.Unknown, Why: found.Why, Line: found.Line}, ""
	}
	var outside, unknown []string
	adequacy := ""
	allAdequate := true
	for _, r := range regions {
		info := lookupRegion(provider, r)
		switch info.where {
		case residencyUnknown:
			unknown = append(unknown, r)
		case residencyAdequate:
			outside = append(outside, r)
			if adequacy == "" {
				adequacy = info.country
			}
		case residencyThird:
			outside = append(outside, r)
			allAdequate = false
		}
	}
	switch {
	case len(outside) > 0:
		why := fmt.Sprintf("region %s is outside the EEA", strings.Join(outside, ", "))
		if found.Why != "" {
			why += " (" + found.Why + ")"
		}
		if !allAdequate {
			adequacy = ""
		}
		return domain.Setting{State: domain.No, Why: why, Line: found.Line}, adequacy
	case len(unknown) > 0:
		return domain.Setting{State: domain.Unknown, Why: fmt.Sprintf("region %s is not a known %s region", strings.Join(unknown, ", "), provider), Line: found.Line}, ""
	}
	why := fmt.Sprintf("region %s is in the EEA", strings.Join(regions, ", "))
	if found.Why != "" {
		why += " (" + found.Why + ")"
	}
	return domain.Setting{State: domain.Yes, Why: why, Line: found.Line}, ""
}
