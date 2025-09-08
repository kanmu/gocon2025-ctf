package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestLoginHandler(t *testing.T) {
	t.Run("GET login page", func(t *testing.T) {
		// Input: GET request to "/"
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(loginHandler)
		handler.ServeHTTP(rr, req)

		// Expected Output: Login page with status 200
		if status := rr.Code; status != http.StatusOK {
			t.Errorf("Expected status %v, got %v", http.StatusOK, status)
		}

		body := rr.Body.String()
		if !strings.Contains(body, "レシピサイト") {
			t.Errorf("Expected login page title, but not found")
		}

		if !strings.Contains(body, `<form action="/login" method="post">`) {
			t.Errorf("Expected login form, but not found")
		}
	})

	t.Run("valid login: kanmu/gocon2025", func(t *testing.T) {
		// Input: POST with valid credentials
		form := url.Values{}
		form.Add("username", "kanmu")
		form.Add("password", "gocon2025")

		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/login", strings.NewReader(form.Encode()))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(loginHandler)
		handler.ServeHTTP(rr, req)

		// Expected Output: Redirect to dashboard with user cookie
		if status := rr.Code; status != http.StatusFound {
			t.Errorf("Expected status %v, got %v", http.StatusFound, status)
		}

		location := rr.Header().Get("Location")
		if location != "/dashboard" {
			t.Errorf("Expected redirect to /dashboard, got %v", location)
		}

		cookies := rr.Header()["Set-Cookie"]
		found := false
		for _, cookie := range cookies {
			if strings.Contains(cookie, "user=kanmu") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected user cookie to be set")
		}
	})

	t.Run("invalid login: invalid/invalid", func(t *testing.T) {
		// Input: POST with invalid credentials
		form := url.Values{}
		form.Add("username", "invalid")
		form.Add("password", "invalid")

		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/login", strings.NewReader(form.Encode()))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(loginHandler)
		handler.ServeHTTP(rr, req)

		// Expected Output: Login page with error message
		if status := rr.Code; status != http.StatusOK {
			t.Errorf("Expected status %v, got %v", http.StatusOK, status)
		}

		body := rr.Body.String()
		if !strings.Contains(body, "ユーザー名またはパスワードが間違っています") {
			t.Errorf("Expected error message, but not found")
		}
	})

	t.Run("SQL injection: admin' OR '1'='1' --/test", func(t *testing.T) {
		// Input: POST with SQL injection payload
		form := url.Values{}
		form.Add("username", "admin' OR '1'='1' --")
		form.Add("password", "test")

		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/login", strings.NewReader(form.Encode()))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(loginHandler)
		handler.ServeHTTP(rr, req)

		// Expected Output: All users data exposed
		if status := rr.Code; status != http.StatusOK {
			t.Errorf("Expected status %v, got %v", http.StatusOK, status)
		}

		body := rr.Body.String()
		expectedUsers := []string{
			"全ユーザー情報",
			"kanmu", "gocon2025",
			"admin", "Adm1n$ecur3",
			"gocon", "G0c0n2025!",
			"vandle", "V@ndl3P@ss",
			"zip", "qwerty123456",
		}

		for _, expected := range expectedUsers {
			if !strings.Contains(body, expected) {
				t.Errorf("Expected '%s' in SQL injection result, but not found", expected)
			}
		}
	})
}

func TestDashboardHandler(t *testing.T) {
	t.Run("without authentication -> redirect to login", func(t *testing.T) {
		// Input: GET /dashboard without cookies
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/dashboard", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(dashboardHandler)
		handler.ServeHTTP(rr, req)

		// Expected Output: Redirect to login page
		if status := rr.Code; status != http.StatusFound {
			t.Errorf("Expected status %v, got %v", http.StatusFound, status)
		}

		location := rr.Header().Get("Location")
		if location != "/" {
			t.Errorf("Expected redirect to /, got %v", location)
		}
	})

	t.Run("kanmu user -> personal dashboard", func(t *testing.T) {
		// Input: GET /dashboard with kanmu cookie
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/dashboard", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.AddCookie(&http.Cookie{Name: "user", Value: "kanmu"})

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(dashboardHandler)
		handler.ServeHTTP(rr, req)

		// Expected Output: Kanmu's personal dashboard with recipes 2,3,5
		if status := rr.Code; status != http.StatusOK {
			t.Errorf("Expected status %v, got %v", http.StatusOK, status)
		}

		body := rr.Body.String()
		expectedContent := []string{
			"kanmuのダッシュボード",
			"ぎょうざ", "いくらとポテト", "ピザ",
			"/recipe/2", "/recipe/3", "/recipe/5",
		}

		for _, expected := range expectedContent {
			if !strings.Contains(body, expected) {
				t.Errorf("Expected '%s' in kanmu dashboard, but not found", expected)
			}
		}
	})

	t.Run("admin user -> generic dashboard", func(t *testing.T) {
		// Input: GET /dashboard with admin cookie
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/dashboard", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.AddCookie(&http.Cookie{Name: "user", Value: "admin"})

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(dashboardHandler)
		handler.ServeHTTP(rr, req)

		// Expected Output: Generic dashboard with recipe 13 only
		if status := rr.Code; status != http.StatusOK {
			t.Errorf("Expected status %v, got %v", http.StatusOK, status)
		}

		body := rr.Body.String()
		expectedContent := []string{
			"レシピダッシュボード",
			"ステーキソース",
			"/recipe/13",
		}

		for _, expected := range expectedContent {
			if !strings.Contains(body, expected) {
				t.Errorf("Expected '%s' in admin dashboard, but not found", expected)
			}
		}
	})
}

func TestRecipeHandler(t *testing.T) {
	t.Run("without authentication -> redirect to login", func(t *testing.T) {
		// Input: GET /recipe/2 without cookies
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/recipe/2", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(recipeHandler)
		handler.ServeHTTP(rr, req)

		// Expected Output: Redirect to login
		if status := rr.Code; status != http.StatusFound {
			t.Errorf("Expected status %v, got %v", http.StatusFound, status)
		}
	})

	t.Run("valid recipe: kanmu accessing recipe/2 -> gyoza details", func(t *testing.T) {
		// Input: GET /recipe/2 with kanmu cookie
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/recipe/2", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.AddCookie(&http.Cookie{Name: "user", Value: "kanmu"})

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(recipeHandler)
		handler.ServeHTTP(rr, req)

		// Expected Output: Gyoza recipe page with cooking steps
		if status := rr.Code; status != http.StatusOK {
			t.Errorf("Expected status %v, got %v", http.StatusOK, status)
		}

		body := rr.Body.String()
		expectedContent := []string{
			"ぎょうざ",
			"豚ひき肉300gに醤油、酒、ごま油を加えて混ぜる",
			"白菜とニラをみじん切りにして塩もみし、水気を絞る",
		}

		for _, expected := range expectedContent {
			if !strings.Contains(body, expected) {
				t.Errorf("Expected '%s' in gyoza recipe, but not found", expected)
			}
		}
	})

	t.Run("invalid recipe: recipe/999 -> not found", func(t *testing.T) {
		// Input: GET /recipe/999 with kanmu cookie
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/recipe/999", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.AddCookie(&http.Cookie{Name: "user", Value: "kanmu"})

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(recipeHandler)
		handler.ServeHTTP(rr, req)

		// Expected Output: 404 Not Found page
		if status := rr.Code; status != http.StatusNotFound {
			t.Errorf("Expected status %v, got %v", http.StatusNotFound, status)
		}

		body := rr.Body.String()
		if !strings.Contains(body, "レシピが見つかりません") {
			t.Errorf("Expected not found message, but not found")
		}
	})

	t.Run("vulnerability tests: unauthorized access should work", func(t *testing.T) {
		testCases := []struct {
			name     string
			user     string
			recipeID string
			expected string
		}{
			{
				name:     "admin accessing kanmu's gyoza",
				user:     "admin",
				recipeID: "2",
				expected: "ぎょうざ",
			},
			{
				name:     "admin accessing hidden sashimi",
				user:     "admin",
				recipeID: "4",
				expected: "さしみ料理",
			},
			{
				name:     "kanmu accessing admin's steak sauce",
				user:     "kanmu",
				recipeID: "13",
				expected: "ステーキソース",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Input: GET /recipe/{id} with different user
				req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/recipe/"+tc.recipeID, nil)
				if err != nil {
					t.Fatal(err)
				}
				req.AddCookie(&http.Cookie{Name: "user", Value: tc.user})

				rr := httptest.NewRecorder()
				handler := http.HandlerFunc(recipeHandler)
				handler.ServeHTTP(rr, req)

				// Expected Output: Access granted due to vulnerability
				if status := rr.Code; status != http.StatusOK {
					t.Errorf("Expected unauthorized access to work, got status %v", status)
				}

				body := rr.Body.String()
				if !strings.Contains(body, tc.expected) {
					t.Errorf("Expected access to %s recipe, but access denied", tc.expected)
				}
			})
		}
	})

	t.Run("image endpoint: recipe/2?format=image -> JPEG image", func(t *testing.T) {
		// Input: GET /recipe/2?format=image with kanmu cookie
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/recipe/2?format=image", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.AddCookie(&http.Cookie{Name: "user", Value: "kanmu"})

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(recipeHandler)
		handler.ServeHTTP(rr, req)

		// Expected Output: JPEG image data
		if status := rr.Code; status != http.StatusOK {
			t.Errorf("Expected status %v, got %v", http.StatusOK, status)
		}

		contentType := rr.Header().Get("Content-Type")
		if contentType != "image/jpeg" {
			t.Errorf("Expected Content-Type image/jpeg, got %v", contentType)
		}

		if rr.Body.Len() == 0 {
			t.Errorf("Expected image data, but got empty response")
		}
	})

	t.Run("download button: recipe/13 -> shows flag download", func(t *testing.T) {
		// Input: GET /recipe/13 with admin cookie
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/recipe/13", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.AddCookie(&http.Cookie{Name: "user", Value: "admin"})

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(recipeHandler)
		handler.ServeHTTP(rr, req)

		// Expected Output: Recipe page with download button
		if status := rr.Code; status != http.StatusOK {
			t.Errorf("Expected status %v, got %v", http.StatusOK, status)
		}

		body := rr.Body.String()
		expectedContent := []string{
			"/download/flag.zip",
			"フラグGet！",
		}

		for _, expected := range expectedContent {
			if !strings.Contains(body, expected) {
				t.Errorf("Expected '%s' in recipe 13, but not found", expected)
			}
		}
	})
}

func TestDownloadHandler(t *testing.T) {
	t.Run("without authentication -> redirect to login", func(t *testing.T) {
		// Input: GET /download/flag.zip without cookies
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/download/flag.zip", nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(downloadHandler)
		handler.ServeHTTP(rr, req)

		// Expected Output: Redirect to login
		if status := rr.Code; status != http.StatusFound {
			t.Errorf("Expected status %v, got %v", http.StatusFound, status)
		}
	})

	t.Run("valid download: admin/flag.zip -> ZIP file", func(t *testing.T) {
		// Input: GET /download/flag.zip with admin cookie
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/download/flag.zip", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.AddCookie(&http.Cookie{Name: "user", Value: "admin"})

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(downloadHandler)
		handler.ServeHTTP(rr, req)

		// Expected Output: ZIP file download with proper headers
		if status := rr.Code; status != http.StatusOK {
			t.Errorf("Expected status %v, got %v", http.StatusOK, status)
		}

		contentType := rr.Header().Get("Content-Type")
		if contentType != "application/zip" {
			t.Errorf("Expected Content-Type application/zip, got %v", contentType)
		}

		disposition := rr.Header().Get("Content-Disposition")
		if !strings.Contains(disposition, `filename="flag.zip"`) {
			t.Errorf("Expected correct Content-Disposition header")
		}

		if rr.Body.Len() == 0 {
			t.Errorf("Expected ZIP file data, but got empty response")
		}
	})

	t.Run("invalid file: admin/invalid.zip -> not found", func(t *testing.T) {
		// Input: GET /download/invalid.zip with admin cookie
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/download/invalid.zip", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.AddCookie(&http.Cookie{Name: "user", Value: "admin"})

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(downloadHandler)
		handler.ServeHTTP(rr, req)

		// Expected Output: 404 Not Found
		if status := rr.Code; status != http.StatusNotFound {
			t.Errorf("Expected status %v, got %v", http.StatusNotFound, status)
		}
	})
}

func TestGetRecipe(t *testing.T) {
	t.Run("existing recipes -> recipe objects", func(t *testing.T) {
		testCases := []struct {
			name     string
			id       int
			expected string
		}{
			{"gyoza recipe", 2, "ぎょうざ"},
			{"ikura potato recipe", 3, "いくらとポテト"},
			{"pizza recipe", 5, "ピザ"},
			{"steak sauce recipe", 13, "ステーキソース"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Input: Recipe ID
				recipe := getRecipe(tc.id)

				// Expected Output: Recipe object with correct name and ID
				if recipe == nil {
					t.Errorf("getRecipe(%d) should not return nil", tc.id)
					return
				}
				if recipe.Name != tc.expected {
					t.Errorf("getRecipe(%d) returned wrong name: got %v want %v", tc.id, recipe.Name, tc.expected)
				}
				if recipe.ID != tc.id {
					t.Errorf("getRecipe(%d) returned wrong ID: got %v want %v", tc.id, recipe.ID, tc.id)
				}
			})
		}
	})

	t.Run("non-existent recipe: ID 999 -> nil", func(t *testing.T) {
		// Input: Non-existent recipe ID
		recipe := getRecipe(999)

		// Expected Output: nil
		if recipe != nil {
			t.Errorf("getRecipe(999) should return nil")
		}
	})
}

func TestGetRecipeDetailData(t *testing.T) {
	t.Run("regular recipes -> detail data without download", func(t *testing.T) {
		testCases := []struct {
			name         string
			id           int
			expectedName string
		}{
			{"gyoza detail", 2, "ぎょうざ"},
			{"ikura potato detail", 3, "いくらとポテト"},
			{"sashimi detail", 4, "さしみ料理"},
			{"pizza detail", 5, "ピザ"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Input: Recipe ID for regular recipe
				recipeDetail := getRecipeDetailData(tc.id)

				// Expected Output: Detail data without download button
				if recipeDetail == nil {
					t.Errorf("getRecipeDetailData(%d) should not return nil", tc.id)
					return
				}
				if recipeDetail.Name != tc.expectedName {
					t.Errorf("Expected name %v, got %v", tc.expectedName, recipeDetail.Name)
				}
				if recipeDetail.ShowDownload != false {
					t.Errorf("Expected ShowDownload false, got %v", recipeDetail.ShowDownload)
				}
				if len(recipeDetail.Steps) == 0 {
					t.Errorf("Expected cooking steps, but got empty")
				}
			})
		}
	})

	t.Run("steak sauce recipe: ID 13 -> detail data with download", func(t *testing.T) {
		// Input: Recipe ID 13 (steak sauce)
		recipeDetail := getRecipeDetailData(13)

		// Expected Output: Detail data with download button enabled
		if recipeDetail == nil {
			t.Errorf("getRecipeDetailData(13) should not return nil")
			return
		}
		if recipeDetail.Name != "ステーキソース" {
			t.Errorf("Expected name ステーキソース, got %v", recipeDetail.Name)
		}
		if recipeDetail.ShowDownload != true {
			t.Errorf("Expected ShowDownload true, got %v", recipeDetail.ShowDownload)
		}
		if len(recipeDetail.Steps) == 0 {
			t.Errorf("Expected cooking steps, but got empty")
		}
	})

	t.Run("non-existent recipe: ID 999 -> nil", func(t *testing.T) {
		// Input: Non-existent recipe ID
		recipeDetail := getRecipeDetailData(999)

		// Expected Output: nil
		if recipeDetail != nil {
			t.Errorf("getRecipeDetailData(999) should return nil")
		}
	})
}

func TestEmbeddedAssets(t *testing.T) {
	t.Run("asset size validation -> all assets non-empty", func(t *testing.T) {
		testCases := []struct {
			name string
			data []byte
		}{
			{"usersCSV", usersCSV},
			{"loginHTML", loginHTML},
			{"notFoundHTML", notFoundHTML},
			{"dashboardHTML", dashboardHTML},
			{"recipeDetailHTML", recipeDetailHTML},
			{"Ingredients", Ingredients},
			{"gyozaImage", gyozaImage},
			{"ikuraPotatoImage", ikuraPotatoImage},
			{"pizzaImage", pizzaImage},
			{"sashimiImage", sashimiImage},
			{"steakSauceImage", steakSauceImage},
		}

		for _, tc := range testCases {
			// Input: Embedded asset data
			// Expected Output: Non-empty data
			if len(tc.data) == 0 {
				t.Errorf("Embedded asset %s is empty", tc.name)
			}
		}
	})

	t.Run("users CSV content -> correct user credentials", func(t *testing.T) {
		// Input: Embedded users CSV data
		// Expected Output: Contains kanmu and zip users with correct passwords
		if !bytes.Contains(usersCSV, []byte("kanmu,gocon2025")) {
			t.Errorf("usersCSV does not contain expected kanmu user")
		}
		if !bytes.Contains(usersCSV, []byte("zip,qwerty123456")) {
			t.Errorf("usersCSV does not contain expected zip user")
		}
	})

	t.Run("HTML template content -> expected UI elements", func(t *testing.T) {
		// Input: Embedded HTML template data
		// Expected Output: Contains expected Japanese text and CSS classes
		if !bytes.Contains(loginHTML, []byte("レシピサイト")) {
			t.Errorf("loginHTML does not contain expected title")
		}
		if !bytes.Contains(dashboardHTML, []byte("レシピコレクション")) {
			t.Errorf("dashboardHTML does not contain expected title")
		}
		if !bytes.Contains(recipeDetailHTML, []byte("recipe-card")) {
			t.Errorf("recipeDetailHTML does not contain expected class")
		}
	})
}
