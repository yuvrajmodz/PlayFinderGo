package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type AppInfo struct {
	Status            string `json:"status"`
	AppName           string `json:"AppName"`
	AppLogo           string `json:"app_logo"`
	PackageName       string `json:"package_name"`
	CompanyName       string `json:"Company_name"`
	CompanyLink       string `json:"CompanyLink"`
	EstimatedDownloads string `json:"Estimated_Downloads"`
	StarRating        string `json:"Star_Rating"`
	TotalReviews      string `json:"Total_Reviews"`
	MinimumAge        string `json:"MinimumAge"`
	Description       string `json:"Description"`
	LastUpdate        string `json:"last_update"`
	ContactSupport    string `json:"Contact_Support"`
	OfficialWebsite   string `json:"Official_Website"`
	DownloadLink      string `json:"DownloadLink"`
}

type ErrorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

var httpClient = &http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse // disable auto-redirect so we handle manually
	},
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "    ")
	_ = enc.Encode(payload)
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, ErrorResponse{Status: "error", Message: message})
}

func fetchPage(targetURL string, mobile bool) (*goquery.Document, error) {
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-IN,en-GB;q=0.9,en-US;q=0.8,en;q=0.7")

	if mobile {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 12; SM-F926B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/107.0.0.0 Safari/537.36")
	} else {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return goquery.NewDocumentFromReader(resp.Body)
}

func searchApp(query string) (string, error) {
	searchURL := "https://play.google.com/store/search?q=" + url.QueryEscape(query) + "&c=apps"

	doc, err := fetchPage(searchURL, false)
	if err != nil {
		return "", fmt.Errorf("failed to search Play Store: %w", err)
	}

	if href, exists := doc.Find("a.Qfxief[href]").First().Attr("href"); exists {
		if strings.HasPrefix(href, "/store/apps/details?id=") {
			parts := strings.SplitN(href, "id=", 2)
			if len(parts) == 2 {
				return parts[1], nil
			}
		}
	}

	if href, exists := doc.Find("a.Si6A0c.Gy4nib[href]").First().Attr("href"); exists {
		if strings.HasPrefix(href, "/store/apps/details?id=") {
			parts := strings.SplitN(href, "id=", 2)
			if len(parts) == 2 {
				return parts[1], nil
			}
		}
	}

	return "", fmt.Errorf("app not found in search results")
}

func scrapeApp(appID string) (*AppInfo, error) {
	pageURL := "https://play.google.com/store/apps/details?id=" + appID

	doc, err := fetchPage(pageURL, true)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch app page: %w", err)
	}

	appName := doc.Find("span[itemprop='name']").First().Text()
	appName = strings.TrimSpace(appName)
	if appName == "" {
		return nil, fmt.Errorf("app not found")
	}

	companyName := "Not Available"
	companyLink := "Not Available"
	companyTag := doc.Find("div.Vbfug.auoIOc").First()
	if companyTag.Length() > 0 {
		companyName = strings.TrimSpace(companyTag.Find("span").First().Text())
		if href, exists := companyTag.Find("a").First().Attr("href"); exists && href != "" {
			companyLink = "https://play.google.com" + href
		}
	}

	starRating := "Not Available"
	ratingTag := doc.Find("div[itemprop='starRating']").First()
	if ratingTag.Length() > 0 {
		raw := strings.TrimSpace(ratingTag.Find("div.TT9eCd").First().Text())
		starRating = strings.TrimSpace(strings.ReplaceAll(raw, "star", ""))
	}

	totalReviews := "Not Available"
	if rev := strings.TrimSpace(doc.Find("div.g1rdde").First().Text()); rev != "" {
		totalReviews = rev
	}

	minimumAge := "Not Available"
	ageTag := doc.Find("span[itemprop='contentRating']").First()
	if ageTag.Length() > 0 {
		if inner := strings.TrimSpace(ageTag.Find("span").First().Text()); inner != "" {
			minimumAge = inner
		}
	}

	description := "Not Available"
	descTag := doc.Find("meta[itemprop='description']").First()
	if content, exists := descTag.Attr("content"); exists && content != "" {
		description = strings.TrimSpace(content)
	}

	lastUpdate := "Not Available"
	if upd := strings.TrimSpace(doc.Find("div.xg1aie").First().Text()); upd != "" {
		lastUpdate = upd
	}

	supportMail := "Not Available"
	if mail := strings.TrimSpace(doc.Find("div.pSEeg").First().Text()); mail != "" {
		supportMail = mail
	}

	appLogo := "Not Available"
	logoTag := doc.Find("img.T75of.cN0oRe.fFmL2e").First()
	if src, exists := logoTag.Attr("src"); exists && src != "" {
		appLogo = src
	}

	officialWebsite := "Not Available"
	doc.Find("a.Si6A0c.RrSxVb[href]").Each(func(_ int, s *goquery.Selection) {
		if officialWebsite != "Not Available" {
			return
		}
		href, _ := s.Attr("href")
		text := s.Text()
		if strings.Contains(href, "http") && strings.Contains(text, "Website") {
			officialWebsite = href
		}
	})

	estimatedDownloads := "Not Available"
	doc.Find("div.wVqUob").Each(func(_ int, s *goquery.Selection) {
		if estimatedDownloads != "Not Available" {
			return
		}
		label := strings.TrimSpace(s.Find("div.g1rdde").Text())
		value := strings.TrimSpace(s.Find("div.ClM7O").Text())
		if strings.Contains(label, "Downloads") && value != "" {
			estimatedDownloads = value
		}
	})

	return &AppInfo{
		Status:             "success",
		AppName:            appName,
		AppLogo:            appLogo,
		PackageName:        appID,
		CompanyName:        companyName,
		CompanyLink:        companyLink,
		EstimatedDownloads: estimatedDownloads,
		StarRating:         starRating,
		TotalReviews:       totalReviews,
		MinimumAge:         minimumAge,
		Description:        description,
		LastUpdate:         lastUpdate,
		ContactSupport:     supportMail,
		OfficialWebsite:    officialWebsite,
		DownloadLink:       "https://play.google.com/store/apps/details?id=" + appID,
	}, nil
}

func getInfoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Only GET method is allowed")
		return
	}

	q := r.URL.Query()
	appID := strings.TrimSpace(q.Get("id"))
	searchQuery := strings.TrimSpace(q.Get("Search"))

	if searchQuery != "" {
		found, err := searchApp(searchQuery)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		http.Redirect(w, r, "/getinfo?id="+url.QueryEscape(found), http.StatusFound)
		return
	}

	if appID == "" {
		writeError(w, http.StatusBadRequest, "Missing parameter: provide 'id' or 'Search'")
		return
	}

	info, err := scrapeApp(appID)
	if err != nil {
		switch err.Error() {
		case "app not found":
			writeError(w, http.StatusNotFound, "App not found")
		default:
			writeError(w, http.StatusInternalServerError, "Unexpected technical issue, please try again later.")
		}
		return
	}

	writeJSON(w, http.StatusOK, info)
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/getinfo", getInfoHandler)

	addr := ":5000"
	log.Printf("Server started on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
