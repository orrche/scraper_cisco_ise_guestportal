package scraper_cisco_ise_guestportal

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Account struct {
	Username           string
	Password           string
	FirstName          string
	LastName           string
	EmailAddress       string
	Company            string
	PersonBeingVisited string
	Token              string
	FromDate           time.Time
	ToDate             time.Time
}

type Config struct {
	Username  string
	Password  string
	PortalURL string
	Portal    string
}

type CreateAccountAttribute struct {
	Value string `json:"value"`
	Name  string `json:"name"`
}

type AccountRaw struct {
	Values    []CreateAccountAttribute `json:"values"`
	GuestType string                   `json:"guestType"`
}

type CreateAccountResponse struct {
	Status     string                   `json:"status"`
	Messages   []string                 `json:"messages"`
	Attributes []CreateAccountAttribute `json:"attributes"`
}

type Session struct {
	token           string
	portalSessionId string
	url_login       string
	client          *http.Client
	config          *Config
}

const userAgent = "Mozilla/5.0 (X11; Linux i686; rv:10.0) Gecko/20100101 Firefox/10.0"

func getFormValue(body []byte, v string) (string, error) {
	p := regexp.MustCompile("name=\"" + v + "\" value=\"([^\"]*)")
	matches := p.FindAllStringSubmatch(string(body), 2)
	token := matches[0][1]
	return token, nil
}

func getFormToken(body []byte) (string, error) {
	return getFormValue(body, "token")
}
func getFormPortalSessionId(body []byte) (string, error) {
	return getFormValue(body, "portalSessionId")
}
func getFormPortal(body []byte) (string, error) {
	return getFormValue(body, "portal")
}

func (session *Session) GetAccountTokens() ([]string, error) {
	type RawD struct {
		Id string `json:"id"`
	}
	type RawData struct {
		Data []RawD `json:"data"`
	}
	form := url.Values{}
	form.Add("token", session.token)
	form.Add("portalSessionId", session.portalSessionId)
	form.Add("meta.searchBy", "")
	form.Add("meta.search", "")
	form.Add("meta.searchStates", "[]")
	form.Add("meta.perPage", "50")
	form.Add("meta.page", "1")
	form.Add("sortOrder", "asc")
	form.Add("sortBy", "username")
	req, err := http.NewRequest("POST", session.config.PortalURL+"sponsorportal/manageGuestsList.action", strings.NewReader(form.Encode()))
	if err != nil {
		return []string{}, err
	}
	req.Header.Set("Referer", session.url_login)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")

	resp, err := session.client.Do(req)
	if err != nil {
		return []string{}, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []string{}, err
	}

	rawData := RawData{}
	err = json.Unmarshal(body, &rawData)
	if err != nil {
		return []string{}, err
	}

	ret := make([]string, 0, len(rawData.Data))
	for _, d := range rawData.Data {
		ret = append(ret, d.Id)
	}

	return ret, nil

}

func addAccountToFormValues(form *url.Values, account *Account) (string, string) {
	fromDate := account.FromDate.Format("2006-01-02")
	fromTime := account.FromDate.Format("15:04")
	from := account.FromDate.Format("01/02/2006+15:04")

	toDate := account.ToDate.Format("2006-01-02")
	toTime := account.ToDate.Format("15:04")
	to := account.ToDate.Format("01/02/2006+15:04")

	form.Add("firstName", account.FirstName)
	form.Add("lastName", account.LastName)
	form.Add("emailAddress", account.EmailAddress)
	form.Add("company", account.Company)
	form.Add("personBeingVisited", account.PersonBeingVisited)
	form.Add("days", "24")
	form.Add("from-date", fromDate)
	form.Add("from-time", fromTime)
	form.Add("to-date", toDate)
	form.Add("to-time", toTime)
	form.Add("location", "Sweden")
	form.Add("guestType", "One-Day")

	return to, from
}

func (session *Session) UpdateAccount(account Account) error {
	form := url.Values{}
	form.Add("token", session.token)
	form.Add("portalSessionId", session.portalSessionId)
	form.Add("selected", account.Token)
	to, from := addAccountToFormValues(&form, &account)

	// To and from can't be url encoded if its to work
	req, _ := http.NewRequest("POST", session.config.PortalURL+"sponsorportal/editGuest.action", strings.NewReader(form.Encode()+"&to="+to+"&from="+from))
	req.Header.Set("Referer", session.url_login)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")

	resp, _ := session.client.Do(req)

	defer resp.Body.Close()
	return nil
}

func (session *Session) CreateAccount(account Account) (string, error) {
	form := url.Values{}
	form.Add("token", session.token)
	form.Add("portalSessionId", session.portalSessionId)
	to, from := addAccountToFormValues(&form, &account)

	// To and from can't be url encoded if its to work
	req, _ := http.NewRequest("POST", session.config.PortalURL+"sponsorportal/createKnown.action", strings.NewReader(form.Encode()+"&to="+to+"&from="+from))
	req.Header.Set("Referer", session.url_login)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")

	resp, _ := session.client.Do(req)

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	createAccountToken := resp.Header.Get("token")
	if err != nil {
		return "", err
	}

	createResp := CreateAccountResponse{}
	err = json.Unmarshal(body, &createResp)
	if err != nil {
		return "", err
	}

	if len(createResp.Attributes) < 1 {
		return "", errors.New("Failure to create account")
	}

	time.Sleep(500 * time.Millisecond)

	form = url.Values{}
	form.Add("token", createAccountToken)
	form.Add("portalSessionId", session.portalSessionId)
	form.Add("guestType", "One-Day")
	req, _ = http.NewRequest("POST", session.config.PortalURL+"sponsorportal/pending.action", strings.NewReader(form.Encode()))
	req.Header.Set("Referer", session.url_login)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")

	resp, _ = session.client.Do(req)

	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return createResp.Attributes[0].Value, nil
}

func (session *Session) GetAccountData(accountToken string) (Account, error) {
	account := Account{}

	form := url.Values{}
	form.Add("token", session.token)
	form.Add("portalSessionId", session.portalSessionId)
	form.Add("selected", accountToken)
	req, err := http.NewRequest("POST", session.config.PortalURL+"/sponsorportal/readGuest.action", strings.NewReader(form.Encode()))
	if err != nil {
		return account, err
	}
	req.Header.Set("Referer", session.url_login)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")

	resp, err := session.client.Do(req)
	if err != nil {
		return account, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return account, err
	}

	accountRaw := AccountRaw{}
	err = json.Unmarshal(body, &accountRaw)
	if err != nil {
		return account, nil
	}

	account.Token = accountToken
	for _, value := range accountRaw.Values {
		switch value.Name {
		case "username":
			account.Username = value.Value
		case "password":
			account.Password = value.Value
		case "toDate":
			account.ToDate, _ = time.ParseInLocation("01/02/2006 15:04", value.Value, time.Local)
		case "fromDate":
			account.FromDate, _ = time.ParseInLocation("01/02/2006 15:04", value.Value, time.Local)
		case "firstName":
			account.FirstName = value.Value
		case "lastName":
			account.LastName = value.Value
		case "emailAddress":
			account.EmailAddress = value.Value
		}
	}

	return account, nil
}

func (session *Session) Logout() error {
	form := url.Values{}
	form.Add("token", session.token)
	form.Add("portalSessionId", session.portalSessionId)
	req, err := http.NewRequest("POST", session.config.PortalURL+"/sponsorportal/Logout.action?portal="+session.config.Portal, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Referer", session.url_login)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")

	resp, err := session.client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return nil
}

func CreateSession(config *Config) (*Session, error) {
	url_login := config.PortalURL + "/sponsorportal/PortalSetup.action?portal=" + config.Portal
	cookieJar, _ := cookiejar.New(nil)

	client := &http.Client{
		Jar: cookieJar,
	}
	req, _ := http.NewRequest("GET", url_login, nil)
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}

	token, err := getFormToken(body)
	portalSessionId, err := getFormPortalSessionId(body)
	portal, err := getFormPortal(body)
	session := Session{token: token, url_login: url_login, client: client, portalSessionId: portalSessionId, config: config}

	form := url.Values{}
	form.Add("token", token)
	form.Add("portal", portal)
	form.Add("portalSessionId", portalSessionId)
	form.Add("user.username", config.Username)
	form.Add("user.password", config.Password)

	req, _ = http.NewRequest("POST", config.PortalURL+"sponsorportal/LoginSubmit.action?from=LOGIN", strings.NewReader(form.Encode()))
	req.Header.Set("Referer", url_login)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")

	resp, _ = client.Do(req)

	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return &session, nil
}
