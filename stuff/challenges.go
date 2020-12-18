package stuff

import (
	"encoding/hex"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Challenge struct {
	ID         string
	Public     bool
	SharedPath string

	HasPassword  bool
	PasswordHash string

	Expires    bool
	ValidUntil time.Time

	HasUploadCountLimit bool
	MaxUploadCount      int
	UploadCount         int

	uploads []*ChallengeUpload
}

type ChallengeUpload struct {
	Time time.Time
	IP   string
}

func (challenge *Challenge) Uploads() []*ChallengeUpload {
	// todo: probably change with actual data storage
	if challenge.uploads == nil {
		return []*ChallengeUpload{}
	}
	return challenge.uploads
}

func (challenge *Challenge) CookieName() string {
	return hex.EncodeToString([]byte(challenge.ID))
}

func (challenge *Challenge) Expired() bool {
	if !challenge.Expires {
		return false
	}

	return time.Now().After(challenge.ValidUntil)
}

func (challenge *Challenge) HitMaxUploadCount() bool {
	return challenge.HasUploadCountLimit && challenge.UploadCount >= challenge.MaxUploadCount
}

func (challenge *Challenge) SetMaxUploadCount(maxUploadCount int) {
	challenge.HasUploadCountLimit = true
	challenge.MaxUploadCount = maxUploadCount
}

func (challenge *Challenge) SetExpirationDate(date time.Time) {
	challenge.Expires = true
	challenge.ValidUntil = date
}

func (challenge *Challenge) SetPassword(password string) error {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	challenge.HasPassword = true
	challenge.PasswordHash = string(passwordHash)
	return nil
}

func (challenge *Challenge) CheckPassword(password string) error {
	return bcrypt.CompareHashAndPassword([]byte(challenge.PasswordHash), []byte(password))
}

func (challenge *Challenge) StorePassword(password string, w http.ResponseWriter, r *http.Request) {
	passwordCookie := &http.Cookie{
		Name:    challenge.CookieName(),
		Path:    "/",
		Value:   hex.EncodeToString([]byte(password)),
		MaxAge:  int(time.Hour.Seconds()),
		Expires: time.Now().Add(time.Hour),
	}
	http.SetCookie(w, passwordCookie)
}

func (challenge *Challenge) Accessible(r *http.Request) bool {
	if challenge.Expired() {
		return false
	}

	if challenge.HitMaxUploadCount() {
		return false
	}

	if challenge.Public {
		return true
	}

	if challenge.HasPassword {
		if cookie, _ := r.Cookie(challenge.CookieName()); cookie != nil {
			if decoded, err := hex.DecodeString(cookie.Value); err == nil {
				return challenge.CheckPassword(string(decoded)) == nil
			}
		}
	}

	return false
}

type ChallengeRepository interface {
	All(limit int, offset int) []*Challenge
	Get(ID string) *Challenge
	Set(challenge *Challenge)
	Remove(challenge *Challenge)
	ReportChallengeUpload(challenge *Challenge, filePath string, request *http.Request)
}

type ArrayChallengeRepository struct {
	challengeIDs []string
	challenges   map[string]*Challenge
}

func (repo *ArrayChallengeRepository) All(limit int, offset int) []*Challenge {
	challengeCount := len(repo.challengeIDs)

	pageStart := offset
	pageEnd := offset + limit

	if pageEnd > challengeCount {
		pageEnd = challengeCount
	}

	challenges := make([]*Challenge, pageEnd-pageStart)

	for i := range challenges {
		challengeID := repo.challengeIDs[pageStart+i]
		challenges[i] = repo.challenges[challengeID]
	}

	return challenges
}

func (repo *ArrayChallengeRepository) Get(ID string) *Challenge {
	challenge, _ := repo.challenges[ID]
	return challenge
}

func (repo *ArrayChallengeRepository) Set(challenge *Challenge) {
	if _, exists := repo.challenges[challenge.ID]; !exists {
		repo.challengeIDs = append(repo.challengeIDs, challenge.ID)
	}
	repo.challenges[challenge.ID] = challenge
}

func (repo *ArrayChallengeRepository) Remove(challenge *Challenge) {
	delete(repo.challenges, challenge.ID)
	for i, id := range repo.challengeIDs {
		if id == challenge.ID {
			repo.challengeIDs = append(repo.challengeIDs[:i], repo.challengeIDs[i+1:]...)
			break
		}
	}
}

func (repo *ArrayChallengeRepository) ReportChallengeUpload(challenge *Challenge, filePath string, request *http.Request) {
	if challenge.uploads == nil {
		challenge.uploads = []*ChallengeUpload{}
	}

	challenge.uploads = append(challenge.uploads, &ChallengeUpload{
		Time: time.Now(),
		IP:   request.RemoteAddr,
	})
	challenge.UploadCount = len(challenge.uploads)
	repo.Set(challenge)
}

func NewArrayChallengeRepository() ChallengeRepository {
	return &ArrayChallengeRepository{
		challengeIDs: []string{},
		challenges:   make(map[string]*Challenge),
	}
}
