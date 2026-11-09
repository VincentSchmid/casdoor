// Copyright 2026 The Casdoor Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package object

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/xorm-io/xorm"
)

func TestCimdOrganizationSignin(t *testing.T) {
	engine, err := xorm.NewEngine("sqlite", filepath.Join(t.TempDir(), "cimd.db"))
	if err != nil {
		t.Fatal(err)
	}
	previousOrmer := ormer
	previousFetchedAt := cimdFetchedAt
	ormer = &Ormer{Engine: engine}
	cimdFetchedAt = map[string]time.Time{}
	t.Cleanup(func() {
		ormer = previousOrmer
		cimdFetchedAt = previousFetchedAt
		engine.Close()
	})
	if err = engine.Sync2(new(Organization), new(Application), new(Provider)); err != nil {
		t.Fatal(err)
	}

	for _, org := range []string{"built-in", "product", "development"} {
		policy := "open"
		if org == "built-in" {
			policy = "disabled"
		}
		_, err = engine.Insert(&Organization{Owner: "admin", Name: org, DefaultApplication: org + "-app", DcrPolicy: policy})
		if err != nil {
			t.Fatal(err)
		}
		app := &Application{
			Owner: "admin", Name: org + "-app", Organization: org,
			Logo: org + "-logo", FormCss: org + "-css", FormCssMobile: org + "-mobile",
			EnableSigninSession: true,
			SigninMethods:       []*SigninMethod{{Name: "Password", Rule: "All"}},
			Providers:           []*ProviderItem{},
		}
		if org == "built-in" {
			app.SigninMethods[0].Rule = "Hide password"
			app.Providers = []*ProviderItem{{Name: "admin-idp", CanSignIn: true}}
		}
		if _, err = engine.Insert(app); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = engine.Insert(&Provider{Owner: "admin", Name: "admin-idp", Category: "OAuth", Type: "OIDC"}); err != nil {
		t.Fatal(err)
	}
	client := &Application{
		Owner: "admin", Name: "shared-client", Organization: "built-in",
		ClientId: "https://client.example/oauth/client.json", IsShared: true,
		Tags: []string{cimdTag}, RedirectUris: []string{"http://127.0.0.1/callback"},
		GrantTypes: []string{"authorization_code"}, DisplayName: "Example client",
		SigninMethods: []*SigninMethod{{Name: "Password", Rule: "Hide password"}},
		Providers:     []*ProviderItem{{Name: "admin-idp", CanSignIn: true}},
	}
	if _, err = engine.Insert(client); err != nil {
		t.Fatal(err)
	}
	cimdFetchedAt[client.ClientId] = time.Now()

	// Exercise the normal cached path too: existing clients must recover without
	// deleting their registration or changing the built-in administrator's app.
	for _, org := range []string{"product", "development", "product"} {
		app, err := GetApplicationByClientIdMetadata(client.ClientId, org, "en")
		if err != nil {
			t.Fatal(err)
		}
		if app.Organization != org || app.OrganizationObj.Name != org || !app.IsPasswordEnabled() {
			t.Fatalf("wrong sign-in policy for %s: organization=%s password=%v", org, app.Organization, app.IsPasswordEnabled())
		}
		if len(app.Providers) != 0 {
			t.Fatalf("%s inherited infrastructure-admin providers", org)
		}
		if app.Logo != org+"-logo" || app.FormCss != org+"-css" || app.FormCssMobile != org+"-mobile" || !app.EnableSigninSession {
			t.Fatalf("%s did not inherit its default application's login presentation", org)
		}
		if app.ClientId != client.ClientId || app.DisplayName != client.DisplayName || !reflect.DeepEqual(app.RedirectUris, client.RedirectUris) || !reflect.DeepEqual(app.GrantTypes, client.GrantTypes) {
			t.Fatal("organization selection changed client metadata")
		}
		if !app.IsRedirectUriValid("http://127.0.0.1:52817/callback") || app.IsRedirectUriValid("https://other.example/callback") {
			t.Fatal("organization selection changed redirect validation")
		}
		loginApp, err := GetApplicationForSignin("admin/"+client.Name, org)
		if err != nil {
			t.Fatal(err)
		}
		if loginApp.Organization != org || !loginApp.IsPasswordEnabled() || len(loginApp.Providers) != 0 {
			t.Fatal("credential submission did not use the authorization page's sign-in policy")
		}
	}

	stored := &Application{Owner: "admin", Name: client.Name}
	if _, err = engine.Get(stored); err != nil {
		t.Fatal(err)
	}
	if stored.Organization != "built-in" || len(stored.Providers) != 1 || stored.Providers[0].Name != "admin-idp" {
		t.Fatal("request-specific sign-in policy was persisted to the shared client")
	}
	if _, err = GetApplicationByClientIdMetadata(client.ClientId, "built-in", "en"); err == nil {
		t.Fatal("built-in DCR policy was bypassed")
	}
	if _, err = GetApplicationForSignin("admin/"+client.Name, "built-in"); err == nil {
		t.Fatal("credential submission bypassed built-in DCR policy")
	}
	adminApp, err := GetApplicationForSignin("admin/built-in-app", "product")
	if err != nil {
		t.Fatal(err)
	}
	if adminApp.Organization != "built-in" || adminApp.IsPasswordEnabled() || len(adminApp.Providers) != 1 {
		t.Fatal("ordinary applications must retain their own sign-in policy")
	}
	// Also repair legacy clients already saved under the selected organization.
	stored.Organization = "product"
	app, err := getCimdApplicationOfOrganization(stored, "product")
	if err != nil || !app.IsPasswordEnabled() || len(app.Providers) != 0 {
		t.Fatalf("same-organization client retained stale sign-in defaults: %v", err)
	}
	stored.Organization = "built-in"
	if _, err = getCimdApplicationOfOrganization(stored, "missing"); err == nil {
		t.Fatal("missing organization must not fall back to built-in sign-in")
	}
	if stored.Organization != "built-in" {
		t.Fatal("organization selection mutated its input")
	}
}
