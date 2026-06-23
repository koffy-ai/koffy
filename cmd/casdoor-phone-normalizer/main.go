package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"koffy/internal/config"

	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
)

func main() {
	cfg := config.Load()
	casdoorsdk.InitConfig(
		cfg.CasdoorEndpoint,
		cfg.CasdoorClientID,
		cfg.CasdoorClientSecret,
		cfg.CasdoorCertificate,
		cfg.CasdoorOrganizationName,
		cfg.CasdoorApplicationName,
	)

	dryRun := !strings.EqualFold(strings.TrimSpace(os.Getenv("CASDOOR_PHONE_NORMALIZE_DRY_RUN")), "false")
	users, err := casdoorsdk.GetUsers()
	if err != nil {
		log.Fatal(err)
	}

	changed := 0
	for _, user := range users {
		if user == nil {
			continue
		}
		phone, countryCode, ok := normalizedCasdoorPhone(user.Phone, user.CountryCode)
		if !ok || phone == user.Phone && countryCode == user.CountryCode {
			continue
		}
		changed++
		fmt.Printf("%s/%s: phone %q -> %q, countryCode %q -> %q\n", user.Owner, user.Name, user.Phone, phone, user.CountryCode, countryCode)
		if dryRun {
			continue
		}
		user.Phone = phone
		user.CountryCode = countryCode
		ok, err := casdoorsdk.UpdateUserForColumns(user, []string{"phone", "countryCode"})
		if err != nil {
			log.Fatalf("update %s/%s: %v", user.Owner, user.Name, err)
		}
		if !ok {
			log.Fatalf("update %s/%s: casdoor returned affected=false", user.Owner, user.Name)
		}
	}

	if dryRun {
		fmt.Printf("dry-run complete: %d user(s) would be updated. Set CASDOOR_PHONE_NORMALIZE_DRY_RUN=false to apply.\n", changed)
		return
	}
	fmt.Printf("complete: updated %d user(s).\n", changed)
}

func normalizedCasdoorPhone(phone string, countryCode string) (string, string, bool) {
	number := strings.TrimSpace(phone)
	number = strings.ReplaceAll(number, " ", "")
	number = strings.ReplaceAll(number, "-", "")
	if strings.HasPrefix(number, "+86") {
		number = strings.TrimPrefix(number, "+86")
	}
	if strings.HasPrefix(number, "86") && len(number) == 13 {
		number = strings.TrimPrefix(number, "86")
	}
	if len(number) != 11 || number[0] != '1' {
		return "", "", false
	}
	for _, char := range number {
		if char < '0' || char > '9' {
			return "", "", false
		}
	}
	return number, "CN", true
}
