package sessions

import (
	"math/rand"
	"net/http"
	"os"
	"testing"

	"github.com/icrowley/fake"
)

type userState int

const (
	loggedOut = userState(0)
	loggedIn  = userState(1)
)

const (
	actionNew = iota
	actionRequest
	actionFindOrCreate
	actionDelete
	actionMAX
)

type testUserInfo struct {
	username string
	state    userState
	t        *testing.T
	header   http.Header
}

// Implement http.ResponseWriter
func (tu *testUserInfo) Write([]byte) (int, error) {
	tu.t.Errorf("Unexpected call to testUserInfo.Write()!")
	return 0, nil
}

func (tu *testUserInfo) WriteHeader(statusCode int) {
	tu.t.Errorf("Unexpected call to testUserInfo.WriteHeader()!")
	return
}

func (tu *testUserInfo) Header() http.Header {
	// HACK: Delete header every time it's requested
	tu.header = make(map[string][]string)
	return tu.header
}

func (tu *testUserInfo) Cookie(cname string) (*http.Cookie, error) {
	if tu.header == nil {
		return nil, http.ErrNoCookie
	}
	tu.header["Cookie"] = tu.header["Set-Cookie"]
	request := http.Request{Header: tu.header}
	return request.Cookie(cname)
}

func randomTest(t *testing.T, uinfo []testUserInfo) bool {

	for i := 0; i < 1000; i++ {
		u := &uinfo[rand.Intn(len(uinfo))]
		action := rand.Intn(actionMAX)
		switch action {
		case actionNew:
			oldcookie, _ := u.Cookie(sessionCookieName)
			_, err := NewSession(u, u.username)
			if err != nil {
				t.Errorf("Making new session for user (%v): %v", *u, err)
				return true
			}
			if newcookie, _ := u.Cookie(sessionCookieName); newcookie == oldcookie {
				t.Errorf("NewSession for %v: Cookie didn't change (%s == %s)", *u, newcookie, oldcookie)
				return true
			}
			u.state = loggedIn
		case actionRequest:
			s := RequestSession(u)
			switch u.state {
			case loggedOut:
				if s != nil {
					t.Errorf("Logged-out user %v expected no session, got %v!",
						*u, *s)
					return true
				}
			case loggedIn:
				if s == nil {
					t.Errorf("Logged-in user %v expected session, got nil!", *u)
					return true
				}
				if s.UserID != u.username {
					t.Errorf("Logged-in user %v got unexpected username %s!", *u, s.UserID)
					return true
				}
			}
		case actionFindOrCreate:
			olds := RequestSession(u)
			// FIXME: 50/50 use a different username
			news, err := FindOrCreateSession(u, u, u.username)
			if err != nil {
				t.Errorf("FindOrCreateSession %v: %v", *u, err)
				return true
			}
			switch u.state {
			case loggedOut:
				if olds != nil || news == nil {
					t.Errorf("Unexpected before/after for %v: %v %v", *u, olds, news)
					return true
				}
				if news.UserID != u.username {
					t.Errorf("Unexpected UserID for %v: %s", *u, news.UserID)
					return true
				}
			case loggedIn:
				if olds == nil || news == nil {
					t.Errorf("Unexpected before/after for %v: %v %v", *u, olds, news)
					return true
				}
				if olds.UserID != u.username {
					t.Errorf("Unexpected UserID for %v: %s", *u, news.UserID)
					return true
				}
				if news.UserID != u.username {
					t.Errorf("Unexpected UserID for %v: %s", *u, news.UserID)
					return true
				}
			}
			u.state = loggedIn

		case actionDelete:
			olds := RequestSession(u)
			if err := DeleteSessionByRequest(u); err != nil {
				t.Errorf("Deleting session by request for %v: %v", *u, err)
				return true
			}
			news := RequestSession(u)
			switch u.state {
			case loggedOut:
				if olds != nil || news != nil {
					t.Errorf("Unexpected before/after for %v: %v %v", *u, olds, news)
					return true
				}
			case loggedIn:
				if olds == nil || news != nil {
					t.Errorf("Unexpected before/after for %v: %v %v", *u, olds, news)
					return true
				}
			}
			u.state = loggedOut
		}
	}
	return false
}

func TestSession(t *testing.T) {
	rand.Seed(4309)

	sfname := os.TempDir() + "/sessions.store"
	t.Logf("Temporary session store filename: %v", sfname)

	// Remove the file first, just in case
	os.Remove(sfname)

	if err := OpenSessionStore(sfname); err != nil {
		t.Errorf("Creating session store: %v", err)
		return
	}

	uinfo := make([]testUserInfo, 10)

	for i := range uinfo {
		uinfo[i].username = fake.UserName()
		uinfo[i].t = t
	}

	if randomTest(t, uinfo) {
		return
	}

	CloseSessionStore()

	if err := OpenSessionStore(sfname); err != nil {
		t.Errorf("Creating session store: %v", err)
		return
	}

	if randomTest(t, uinfo) {
		return
	}

	// Only remove the file if we were successful
	os.Remove(sfname)
}
