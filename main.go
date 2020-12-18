package main

//go:generate go get -u github.com/valyala/quicktemplate/qtc
//go:generate qtc -dir=templates

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"sort"
	"strconv"
	"time"

	"github.com/AlbinoDrought/creamy-inbound-stuff/stuff"
	"github.com/AlbinoDrought/creamy-inbound-stuff/templates"
	"github.com/julienschmidt/httprouter"
)

const challengeIDLength = 64
const challengeRandomPasswordLength = 128

var dataDirectory = "data"

var challengeRepository stuff.ChallengeRepository
var challengeURLGenerator ChallengeURLGenerator
var browseURLGenerator BrowseURLGenerator

func init() {
	challengeRepository = stuff.NewArrayChallengeRepository()

	urlGenerator := &hardcodedURLGenerator{}
	challengeURLGenerator = urlGenerator
	browseURLGenerator = urlGenerator
}

func writeMessagePage(w http.ResponseWriter, page *templates.MessagePage) {
	w.WriteHeader(page.Status)
	templates.WritePageTemplate(w, page, &templates.EmptyNav{})
}

func renderServerError(w http.ResponseWriter, r *http.Request, err error) {
	writeMessagePage(w, &templates.MessagePage{
		Status: http.StatusInternalServerError,
		Text:   "Internal Server Error",
	})
}

func renderUnauthorized(w http.ResponseWriter, r *http.Request) {
	writeMessagePage(w, &templates.MessagePage{
		Status: http.StatusUnauthorized,
		Text:   "Unauthorized",
	})
}

func renderChallengeNotFound(w http.ResponseWriter, r *http.Request, ID string) {
	writeMessagePage(w, &templates.MessagePage{
		Status: http.StatusNotFound,
		Text:   "Challenge not found",
	})
}

func handleChallengesIndex(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// todo: allow controlling pagination
	challenges := challengeRepository.All(10, 0)

	challengeResources := make([]*templates.ChallengeResource, len(challenges))
	for i, challenge := range challenges {
		challengeResources[i] = &templates.ChallengeResource{
			Challenge: challenge,

			ViewLink: challengeURLGenerator.ViewChallenge(challenge),
		}
	}

	csrfToken, err := getOrCreateCSRF(w, r)
	if err != nil {
		log.Printf("Error with getOrCreateCSRF: %v", err)
		renderServerError(w, r, err)
		return
	}

	templates.WritePageTemplate(w, &templates.ChallengeIndexPage{
		Challenges: challengeResources,
		CSRF:       csrfToken,

		Page: 1,
	}, &templates.PrivateNav{})
}

func handleChallengeDelete(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	challengeID := ps.ByName("challenge")

	if err := validCSRF(r, r.FormValue("_token")); err != nil {
		log.Printf("Error validating CSRF token: %v", err)
		renderServerError(w, r, err)
		return
	}

	challenge := challengeRepository.Get(challengeID)
	if challenge == nil {
		renderChallengeNotFound(w, r, challengeID)
		return
	}

	challengeRepository.Remove(challenge)
	http.Redirect(w, r, "/challenges", http.StatusFound)
}

func handleStuffIndex(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	filePath := path.Clean(ps.ByName("filepath"))

	dir := http.Dir(dataDirectory)
	file, err := dir.Open(filePath)
	if err != nil {
		log.Printf("Error opening file %v: %v", filePath, err)
		renderServerError(w, r, err)
		return
	}

	stat, err := file.Stat()
	if err != nil {
		log.Printf("Error stat'ing file %v: %v", filePath, err)
		renderServerError(w, r, err)
		return
	}

	if !stat.IsDir() {
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", stat.Name()))
		http.ServeFile(w, r, path.Join(dataDirectory, filePath))
		return
	}

	dirs, err := file.Readdir(-1)
	if err != nil {
		log.Printf("Error reading directory %v: %v", filePath, err)
		renderServerError(w, r, err)
		return
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name() < dirs[j].Name() })

	files := make([]templates.File, len(dirs))
	for i, dir := range dirs {
		name := dir.Name()
		if dir.IsDir() {
			name += "/"
		}

		pathRelativeToDataDir := path.Join(filePath, name)

		files[i].Label = name
		files[i].BrowseLink = browseURLGenerator.BrowsePath(pathRelativeToDataDir)
		files[i].ShareLink = browseURLGenerator.SharePath(pathRelativeToDataDir)
	}

	atRoot := filePath == "" || filePath == "/" || filePath == "."
	directoryName := filePath
	if atRoot {
		directoryName = "/"
	}

	browsePage := &templates.BrowsePage{
		DirectoryName: directoryName,
		Files:         files,

		CanTravelUpwards: !atRoot,
		UpwardsLink:      browseURLGenerator.BrowsePath(path.Join(filePath, "..")),
	}
	templates.WritePageTemplate(w, browsePage, &templates.PrivateNav{})
}

func handleStuffShowForm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	filePath := path.Clean(ps.ByName("filepath"))

	dir := http.Dir(dataDirectory)
	_, err := dir.Open(filePath)
	if err != nil {
		log.Printf("Error opening file %v: %v", filePath, err)
		renderServerError(w, r, err)
		return
	}

	csrfToken, err := getOrCreateCSRF(w, r)
	if err != nil {
		log.Printf("Error with getOrCreateCSRF: %v", err)
		renderServerError(w, r, err)
		return
	}

	randomPassword, err := RandomString(challengeRandomPasswordLength)
	if err != nil {
		log.Printf("Error generating random challenge password: %v", err)
		renderServerError(w, r, err)
		return
	}

	sharePage := &templates.SharePage{
		Path:           filePath,
		CSRF:           csrfToken,
		RandomPassword: randomPassword,

		CancelLink: browseURLGenerator.BrowsePath(path.Join(filePath, "..")),
	}
	templates.WritePageTemplate(w, sharePage, &templates.PrivateNav{})
}

func handleStuffReceiveForm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	filePath := path.Clean(ps.ByName("filepath"))

	dir := http.Dir(dataDirectory)
	_, err := dir.Open(filePath)
	if err != nil {
		log.Printf("Error opening file %v: %v", filePath, err)
		renderServerError(w, r, err)
		return
	}

	if err := validCSRF(r, r.FormValue("_token")); err != nil {
		log.Printf("Error validating CSRF token: %v", err)
		renderServerError(w, r, err)
		return
	}

	challengeID, err := RandomString(challengeIDLength)
	if err != nil {
		log.Printf("Error generating challenge ID: %v", err)
		renderServerError(w, r, err)
		return
	}

	challenge := &stuff.Challenge{
		ID:         challengeID,
		Public:     r.FormValue("public") == "1",
		SharedPath: filePath,
	}
	if challengePassword := r.FormValue("challenge-password"); challengePassword != "" {
		if err = challenge.SetPassword(challengePassword); err != nil {
			log.Printf("Error setting challenge password: %v", err)
			renderServerError(w, r, err)
			return
		}
	}
	if expires := r.FormValue("expires"); expires == "1" {
		expirationDate := r.FormValue("expiration-date")
		if expirationDate == "" {
			expirationDate = time.Now().Add(24 * time.Hour).Format("2006-01-02")
		}
		expirationTime := r.FormValue("expiration-time")
		if expirationTime == "" {
			expirationTime = time.Now().Format("15:04")
		}

		parsedExpirationTime, err := time.Parse("2006-01-02 15:04", expirationDate+" "+expirationTime)
		if err != nil {
			log.Printf("Error parsing expiration time: %v", err)
			renderServerError(w, r, err)
			return
		}

		challenge.SetExpirationDate(parsedExpirationTime)
	}
	if maxUploadCountEnabled := r.FormValue("max-upload-count-enabled"); maxUploadCountEnabled == "1" {
		maxUploadCount, err := strconv.Atoi(r.FormValue("max-upload-count"))
		if err != nil {
			log.Printf("Error converting max upload count %s to int: %v", r.FormValue("max-upload-count"), err)
			renderServerError(w, r, err)
			return
		}
		challenge.SetMaxUploadCount(maxUploadCount)
	}

	challengeRepository.Set(challenge)

	sharedChallengePage := &templates.SharedChallengePage{
		Challenge: challenge,

		ViewLink: challengeURLGenerator.ViewChallenge(challenge),
	}
	templates.WritePageTemplate(w, sharedChallengePage, &templates.PrivateNav{})
}

func handleChallengeShowForm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	challengeID := ps.ByName("challenge")

	challenge := challengeRepository.Get(challengeID)
	if challenge == nil {
		renderChallengeNotFound(w, r, challengeID)
		return
	}

	csrfToken, err := getOrCreateCSRF(w, r)
	if err != nil {
		log.Printf("Error with getOrCreateCSRF: %v", err)
		renderServerError(w, r, err)
		return
	}

	if !challenge.Accessible(r) {
		if challenge.HasPassword {
			w.WriteHeader(http.StatusUnauthorized)
			templates.WritePageTemplate(w, &templates.UnlockPage{
				Challenge: challenge,
				CSRF:      csrfToken,
			}, &templates.EmptyNav{})
		} else {
			renderUnauthorized(w, r)
		}

		return
	}

	uploadPage := &templates.UploadPage{
		Challenge: challenge,
		CSRF:      csrfToken,

		UploadURL: challengeURLGenerator.UploadToChallenge(challenge),
	}
	templates.WritePageTemplate(w, uploadPage, &templates.EmptyNav{})
}

func handleChallengeAuth(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	challengeID := ps.ByName("challenge")

	challenge := challengeRepository.Get(challengeID)
	if challenge == nil {
		renderChallengeNotFound(w, r, challengeID)
		return
	}

	// already has access, no need for auth
	if challenge.Accessible(r) {
		http.Redirect(w, r, challengeURLGenerator.UploadToChallenge(challenge), http.StatusFound)
		return
	}

	if err := validCSRF(r, r.FormValue("_token")); err != nil {
		log.Printf("Error validating CSRF token: %v", err)
		renderServerError(w, r, err)
		return
	}

	if challenge.HasPassword {
		postedPassword := r.FormValue("challenge-password")
		if challenge.CheckPassword(postedPassword) == nil {
			challenge.StorePassword(postedPassword, w, r)
			http.Redirect(w, r, r.URL.String(), http.StatusFound)
			return
		}
	}

	handleChallengeShowForm(w, r, ps)
}

func handleChallengeUpload(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	challengeID := ps.ByName("challenge")

	challenge := challengeRepository.Get(challengeID)
	if challenge == nil {
		renderChallengeNotFound(w, r, challengeID)
		return
	}

	if !challenge.Accessible(r) {
		handleChallengeShowForm(w, r, ps)
		return
	}

	if err := validCSRF(r, r.FormValue("_token")); err != nil {
		log.Printf("Error validating CSRF token: %v", err)
		renderServerError(w, r, err)
		return
	}

	inputFile, fileHeader, err := r.FormFile("file")
	if err != nil {
		log.Printf("Error grabbing FormFile: %v", err)
		renderServerError(w, r, err)
		return
	}
	defer inputFile.Close()

	var filename string
	if fileHeader == nil {
		filename = "file.bin"
	} else {
		filename = fileHeader.Filename
	}

	filePath := path.Join(dataDirectory, path.Clean(challenge.SharedPath), path.Clean(path.Base(filename)))

	outputFile, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, os.ModePerm)
	if err != nil {
		if os.IsExist(err) {
			log.Printf("Filepath conflict when trying to open %v: %v", filePath, err)
			writeMessagePage(w, &templates.MessagePage{
				Status: http.StatusConflict,
				Text:   "A file with the same name already exists",
			})
		} else {
			log.Printf("Error creating output file %v: %v", filePath, err)
			renderServerError(w, r, err)
		}
		return
	}
	defer outputFile.Close()

	if _, err = io.Copy(outputFile, inputFile); err != nil {
		log.Printf("Error copying to output file %v: %v", filePath, err)
		renderServerError(w, r, err)
		return
	}

	writeMessagePage(w, &templates.MessagePage{
		Status: http.StatusOK,
		Text:   "File uploaded!",
	})
}

func handleHome(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	templates.WritePageTemplate(w, &templates.HomePage{}, &templates.PrivateNav{})
}

func main() {
	router := httprouter.New()

	router.NotFound = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeMessagePage(w, &templates.MessagePage{
			Status: http.StatusNotFound,
			Text:   "Page Not Found",
		})
	})

	router.MethodNotAllowed = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeMessagePage(w, &templates.MessagePage{
			Status: http.StatusMethodNotAllowed,
			Text:   "Method Not Allowed",
		})
	})

	router.GET("/", handleHome)

	router.GET("/challenges", handleChallengesIndex)
	router.DELETE("/challenges/:challenge", handleChallengeDelete)
	router.POST("/challenges/:challenge/delete", handleChallengeDelete)

	router.GET("/stuff/browse/*filepath", handleStuffIndex)
	router.GET("/stuff/share/*filepath", handleStuffShowForm)
	router.POST("/stuff/share/*filepath", handleStuffReceiveForm)

	router.GET("/upload/:challenge", handleChallengeShowForm)
	router.POST("/upload/:challenge", handleChallengeAuth)
	router.POST("/upload/:challenge/file", handleChallengeUpload)

	log.Fatal(http.ListenAndServe(":8080", router))
}
