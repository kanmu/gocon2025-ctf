// Package main implements a CTF web application for Go Conference 2025.
package main

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/nao1215/filesql"
)

//go:embed assets/users.csv
var usersCSV []byte

//go:embed assets/login.html
var loginHTML []byte

//go:embed assets/not_found.html
var notFoundHTML []byte

//go:embed assets/ingredients_list.zip
var Ingredients []byte

//go:embed assets/dashboard.html
var dashboardHTML []byte

//go:embed assets/recipe_detail.html
var recipeDetailHTML []byte

//go:embed assets/gyoza.jpg
var gyozaImage []byte

//go:embed assets/ikura_to_potato.jpg
var ikuraPotatoImage []byte

//go:embed assets/pizza.jpg
var pizzaImage []byte

//go:embed assets/sashimi.jpg
var sashimiImage []byte

//go:embed assets/steak_sauce.jpg
var steakSauceImage []byte

type LoginData struct {
	Error string
}

type User struct {
	Username string
	Password string
}

const kanmuUser = "kanmu"

type Recipe struct {
	ID          int
	Name        string
	Description string
	Emoji       string
	Image       []byte
	ContentType string
	Steps       []string
}

type DashboardRecipe struct {
	ID          int
	Name        string
	Description string
	Emoji       string
}

type DashboardData struct {
	Title          string
	WelcomeMessage string
	Recipes        []DashboardRecipe
}

type RecipeDetailData struct {
	ID           int
	Name         string
	Description  string
	Emoji        string
	Steps        []string
	ShowDownload bool
}

// Constants and utilities
const (
	flagFilename  = "flag.zip"
	tmpFilePrefix = "/tmp/users.csv"
)

// renderTemplate renders a template with data and handles errors
func renderTemplate(w http.ResponseWriter, templateData []byte, data any, templateName string) error {
	tmpl, err := template.New(templateName).Parse(string(templateData))
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return tmpl.Execute(w, data)
}

// requireAuth checks for authentication and redirects if not authenticated
func requireAuth(w http.ResponseWriter, r *http.Request) (string, bool) {
	cookie, err := r.Cookie("user")
	if err != nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return "", false
	}
	return cookie.Value, true
}

// createTempDatabase creates temporary database file and returns connection
func createTempDatabase() (*sql.DB, string, error) {
	tmpFile := tmpFilePrefix
	if err := os.WriteFile(tmpFile, usersCSV, 0600); err != nil {
		return nil, "", err
	}

	db, err := filesql.Open(tmpFile)
	if err != nil {
		_ = os.Remove(tmpFile) //nolint:errcheck // Temp file cleanup
		return nil, "", err
	}
	return db, tmpFile, nil
}

// cleanup removes temporary file and closes database
func cleanup(db *sql.DB, tmpFile string) {
	if db != nil {
		_ = db.Close()
	}
	_ = os.Remove(tmpFile) //nolint:errcheck // Temp file cleanup
}

func main() {
	http.HandleFunc("/", loginHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/dashboard", dashboardHandler)
	http.HandleFunc("/recipe/", recipeHandler)
	http.HandleFunc("/download/", downloadHandler)

	fmt.Println("Server starting on http://localhost:8080")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// recipeDatabase contains all recipe data
var recipeDatabase = map[int]*Recipe{
	2: {
		ID:          2,
		Name:        "ぎょうざ",
		Description: "パリッとした食感が楽しめる手作りぎょうざ。キャベツとニラの旨みが詰まった定番の中華料理です。",
		Emoji:       "🥟",
		Image:       gyozaImage,
		ContentType: "image/jpeg",
		Steps: []string{
			"豚ひき肉300gに醤油、酒、ごま油を加えて混ぜる",
			"白菜とニラをみじん切りにして塩もみし、水気を絞る",
			"肉と野菜を混ぜ合わせて餡を作る",
			"ぎょうざの皮に餡を包む",
			"フライパンに油を熱し、ぎょうざを並べる",
			"底面に焼き色がついたら水を加えて蓋をし、蒸し焼きにする",
		},
	},
	3: {
		ID:          3,
		Name:        "いくらとポテト",
		Description: "プチプチのいくらとホクホクポテトの贅沢な組み合わせ。見た目も美しく、特別な日にぴったりの一品です。",
		Emoji:       "🥔",
		Image:       ikuraPotatoImage,
		ContentType: "image/jpeg",
		Steps: []string{
			"じゃがいも4個を皮付きのまま茹でる",
			"竹串がスッと通るまで20-25分茹でる",
			"茹で上がったらすぐに冷水で冷やし、皮をむく",
			"適当な大きさに切って器に盛る",
			"いくら50gを上に乗せる",
			"お好みでバターと塩コショウで味付けする",
		},
	},
	4: {
		ID:          4,
		Name:        "さしみ料理",
		Description: "新鮮な魚の旨みを存分に味わえる日本料理の代表格。包丁使いと盛り付けが美しさの決め手です。",
		Emoji:       "🍣",
		Image:       sashimiImage,
		ContentType: "image/jpeg",
		Steps: []string{
			"新鮮な刺身用の魚を用意する",
			"包丁を研いで切れ味を良くする",
			"魚を適当な厚さに切る",
			"わさびと醤油を添える",
			"大根のつまと一緒に盛り付ける",
			"美しく器に盛って完成",
		},
	},
	5: {
		ID:          5,
		Name:        "ピザ",
		Description: "手作り生地で作る本格的なマルゲリータピザ。トマトソースとモッツァレラチーズのシンプルな美味しさ。",
		Emoji:       "🍕",
		Image:       pizzaImage,
		ContentType: "image/jpeg",
		Steps: []string{
			"強力粉200g、薄力粉50g、塩小さじ1を混ぜる",
			"ぬるま湯140mlにドライイースト3gを溶かす",
			"粉類にイースト水を加えてこね、15分発酵させる",
			"生地を薄く伸ばしてピザソースを塗る",
			"チーズとお好みの具材をのせる",
			"220度のオーブンで12-15分焼く",
		},
	},
	13: {
		ID:          13,
		Name:        "ステーキソース",
		Description: "お肉を引き立てる特製ソース。玉ねぎ、りんご、にんにくの絶妙なバランスで、ステーキが格段に美味しくなります！",
		Emoji:       "🥩",
		Image:       steakSauceImage,
		ContentType: "image/jpeg",
		Steps: []string{
			"玉ねぎを炒める",
			"りんごを加える",
			"にんにくを加える",
			"醤油を加える",
			"みりんを加えて煮詰める",
			"全てが混ざり合い、とろみがついたら完成",
		},
	},
}

func getRecipe(id int) *Recipe {
	return recipeDatabase[id]
}

func getRecipeDetailData(id int) *RecipeDetailData {
	recipe := recipeDatabase[id]
	if recipe == nil {
		return nil
	}

	return &RecipeDetailData{
		ID:           recipe.ID,
		Name:         recipe.Name,
		Description:  recipe.Description,
		Emoji:        recipe.Emoji,
		Steps:        recipe.Steps,
		ShowDownload: id == 13, // Only steak sauce recipe shows download
	}
}

// Moved above - now uses recipeDatabase

func recipeHandler(w http.ResponseWriter, r *http.Request) {
	_, authenticated := requireAuth(w, r)
	if !authenticated {
		return
	}

	path := r.URL.Path
	idStr := strings.TrimPrefix(path, "/recipe/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		showNotFound(w)
		return
	}

	recipe := getRecipe(id)
	if recipe == nil {
		showNotFound(w)
		return
	}

	// Authentication already verified by requireAuth

	if strings.Contains(r.URL.Query().Get("format"), "image") {
		w.Header().Set("Content-Type", recipe.ContentType)
		if _, err := w.Write(recipe.Image); err != nil {
			http.Error(w, "Image Error", http.StatusInternalServerError)
		}
		return
	}

	// レシピ詳細データを取得
	recipeDetail := getRecipeDetailData(id)
	if recipeDetail == nil {
		showNotFound(w)
		return
	}

	if err := renderTemplate(w, recipeDetailHTML, recipeDetail, "recipe"); err != nil {
		http.Error(w, "Template Error", http.StatusInternalServerError)
	}
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	_, authenticated := requireAuth(w, r)
	if !authenticated {
		return
	}

	path := r.URL.Path
	filename := strings.TrimPrefix(path, "/download/")

	// Authentication already verified by requireAuth
	if filename == flagFilename {
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", flagFilename))
		w.Header().Set("Content-Length", strconv.Itoa(len(Ingredients)))

		if _, err := w.Write(Ingredients); err != nil {
			http.Error(w, "Download Error", http.StatusInternalServerError)
		}
		return
	}

	showNotFound(w)
}

func showNotFound(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	if _, err := w.Write(notFoundHTML); err != nil {
		http.Error(w, "Template Error", http.StatusInternalServerError)
	}
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if err := renderTemplate(w, loginHTML, LoginData{}, "login"); err != nil {
			http.Error(w, "Template Error", http.StatusInternalServerError)
		}
		return
	}

	if r.Method == http.MethodPost {
		username := r.FormValue("username")
		password := r.FormValue("password")

		db, tmpFile, err := createTempDatabase()
		if err != nil {
			http.Error(w, "Database Error", http.StatusInternalServerError)
			return
		}
		defer cleanup(db, tmpFile)

		query := fmt.Sprintf("SELECT username, password FROM users WHERE username='%s' AND password='%s'", username, password)
		rows, err := db.QueryContext(context.Background(), query)
		if err != nil {
			http.Error(w, "Database Error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var users []User
		for rows.Next() {
			var user User
			if err := rows.Scan(&user.Username, &user.Password); err != nil {
				continue
			}
			users = append(users, user)
		}

		if err := rows.Err(); err != nil {
			http.Error(w, "Database Error", http.StatusInternalServerError)
			return
		}

		if len(users) == 0 {
			if err := renderTemplate(w, loginHTML, LoginData{Error: "ユーザー名またはパスワードが間違っています"}, "login"); err != nil {
				http.Error(w, "Template Error", http.StatusInternalServerError)
			}
			return
		}

		if len(users) > 1 {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, "<h1>全ユーザー情報</h1><table border='1'><tr><th>ユーザー名</th><th>パスワード</th></tr>")
			for _, user := range users {
				fmt.Fprintf(w, "<tr><td>%s</td><td>%s</td></tr>", user.Username, user.Password)
			}
			fmt.Fprint(w, "</table>")
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:  "user",
			Value: users[0].Username,
			Path:  "/",
		})
		http.Redirect(w, r, "/dashboard", http.StatusFound)
	}
}

func dashboardHandler(w http.ResponseWriter, r *http.Request) {
	user, authenticated := requireAuth(w, r)
	if !authenticated {
		return
	}

	var data DashboardData

	if user == kanmuUser {
		data = DashboardData{
			Title:          "kanmuのダッシュボード",
			WelcomeMessage: fmt.Sprintf("🎉 こんにちは、%sさん！あなたの美味しいレシピコレクションをお楽しみください。", user),
			Recipes: []DashboardRecipe{
				{ID: 2, Name: "ぎょうざ", Description: "パリッとした食感が楽しめる手作りぎょうざ", Emoji: "🥟"},
				{ID: 3, Name: "いくらとポテト", Description: "プチプチのいくらとホクホクポテトの贅沢な一品", Emoji: "🥔"},
				{ID: 5, Name: "ピザ", Description: "手作り生地で作る本格的なマルゲリータピザ", Emoji: "🍕"},
			},
		}
	} else {
		data = DashboardData{
			Title:          "レシピダッシュボード",
			WelcomeMessage: fmt.Sprintf("✨ こんにちは、%sさん！利用可能なレシピをご覧ください。", user),
			Recipes: []DashboardRecipe{
				{ID: 13, Name: "ステーキソース", Description: "お肉を引き立てる特製ソース。隠し味で絶品に！", Emoji: "🥩"},
			},
		}
	}

	if err := renderTemplate(w, dashboardHTML, data, "dashboard"); err != nil {
		http.Error(w, "Template Error", http.StatusInternalServerError)
	}
}
