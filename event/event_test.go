package event

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"testing"

	"github.com/icrowley/fake"
)

func TestVersion(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "event")
	if err != nil {
		t.Errorf("Creating temporary directory: %v", err)
		return
	}

	sfname := tmpdir + "/event.sqlite3"
	t.Logf("Temporary session store filename: %v", sfname)

	// Test simple open / close
	db, err := openDb(sfname)
	if err != nil {
		t.Errorf("Opening database: %v", err)
		return
	}

	db.Close()

	db, err = openDb(sfname)
	if err != nil {
		t.Errorf("Opening database a second time: %v", err)
		return
	}

	// Manually break the schema version
	_, err = db.Exec(fmt.Sprintf("pragma user_version=%d", codeSchemaVersion+1))
	if err != nil {
		t.Errorf("Messing up user version: %v", err)
		return
	}

	db.Close()

	db, err = openDb(sfname)
	if err == nil {
		t.Errorf("Opening database with wrong version succeeded!")
		return
	}

	os.RemoveAll(tmpdir)
}

// This password should always be suitable
const TestPassword = "xenuser"

func testNewUser(t *testing.T) (User, bool) {
	user := User{
		Username:    fake.UserName(),
		IsVerified:  false,
		RealName:    fake.FullName(),
		Company:     fake.Company(),
		Email:       fake.EmailAddress(),
		Description: fake.Paragraphs(),
	}

	t.Logf("Creating test user %v", user)

	var err error
	for _, err = NewUser(TestPassword, &user); err != nil; _, err = NewUser(TestPassword, &user) {
		// Just keep trying random usernames until we get a new one
		if err == errUsernameExists {
			user.Username = fake.UserName()
			t.Logf(" User exists!  Trying username %s instead", user.Username)
			continue
		}
		t.Errorf("Creating a test user: %v", err)
		return user, true
	}

	return user, false
}

func compareUsers(u1, u2 *User, t *testing.T) bool {
	ret := true
	if u1.UserID != u2.UserID {
		t.Logf("mismatch UserID: %v != %v", u1.UserID, u2.UserID)
		ret = false
	}
	if u1.Username != u2.Username {
		t.Logf("mismatch Username: %v != %v", u1.Username, u2.Username)
		ret = false
	}
	if u1.IsAdmin != u2.IsAdmin {
		t.Logf("mismatch IsAdmin: %v != %v", u1.IsAdmin, u2.IsAdmin)
		ret = false
	}
	if u1.IsVerified != u2.IsVerified {
		t.Logf("mismatch IsVerified: %v != %v", u1.IsVerified, u2.IsVerified)
		ret = false
	}
	if u1.RealName != u2.RealName {
		t.Logf("mismatch RealName: %v != %v", u1.RealName, u2.RealName)
		ret = false
	}
	if u1.Email != u2.Email {
		t.Logf("mismatch Email: %v != %v", u1.Email, u2.Email)
		ret = false
	}
	if u1.Company != u2.Company {
		t.Logf("mismatch Company: %v != %v", u1.Company, u2.Company)
		ret = false
	}
	if u1.Description != u2.Description {
		t.Logf("mismatch Description: %v != %v", u1.Description, u2.Company)
		ret = false
	}
	return ret
}

func TestUnitUser(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "event")
	if err != nil {
		t.Errorf("Creating temporary directory: %v", err)
		return
	}

	jsonfname := tmpdir + "/event.json"
	dbfname := tmpdir + "/event.sqlite3"
	t.Logf("Temporary session store filenames: %s, %s", jsonfname, dbfname)

	// Remove the file first, just in case
	os.Remove(jsonfname)
	os.Remove(dbfname)

	// Test simple open / close
	err = Load(EventOptions{storeFilename: jsonfname, dbFilename: dbfname})
	if err != nil {
		t.Errorf("Opening stores: %v", err)
		return
	}

	t.Logf("Testing UserFindRandom when there are no users")
	{
		gotuser, err := UserFindRandom()
		if err != nil {
			t.Errorf("ERROR: UserFindRandom: %v", err)
			return
		}
		if gotuser != nil {
			t.Errorf("ERROR: UserFindRandom: Expected nil, got %v!", gotuser)
			return
		}
	}

	// Make 5 users, and use them to unit-test various functions
	testUserCount := 5
	users := make([]User, testUserCount)

	for i := range users {
		exit := false
		users[i], exit = testNewUser(t)
		if exit {
			return
		}

		// Try creating a new user with the same userid
		userCopy := users[i]
		_, err = NewUser(TestPassword, &userCopy)
		if err != errUsernameExists {
			t.Errorf("Testing duplicate username: expected errUsernameExists, got %v!", err)
			return
		}

		// Look for that user by uid
		gotuser, err := UserFind(users[i].UserID)
		if err != nil {
			t.Errorf("Finding the user we just created by ID: %v", err)
			return
		}
		if gotuser == nil {
			t.Errorf("Couldn't find just-created user by id %s!", users[i].UserID)
			return
		}

		if !compareUsers(&users[i], gotuser, t) {
			t.Errorf("User data mismatch")
			return
		}

		if !gotuser.CheckPassword(TestPassword) {
			t.Errorf("Password failed")
			return
		}

		// Look for that user by username
		gotuser, err = UserFindByUsername(users[i].Username)
		if err != nil {
			t.Errorf("Finding the user we just created by username: %v", err)
			return
		}

		if !compareUsers(&users[i], gotuser, t) {
			t.Errorf("User data mismatch")
			return
		}

		// Look for that user by username, checking password
		gotuser, err = UserFindByUsername(users[i].Username)
		if err != nil {
			t.Errorf("Finding the user we just created by username: %v", err)
			return
		}

		if !compareUsers(&users[i], gotuser, t) {
			t.Errorf("User data mismatch")
			return
		}

		gotuser, err = UserFindRandom()
		if err != nil {
			t.Errorf("Finding a random user: %v", err)
			return
		}

		// If this is our first user, there's only one possibility
		if i == 0 && !compareUsers(&users[i], gotuser, t) {
			t.Errorf("Random user didn't return the only non-admin user!")
			return
		}

		if gotuser.Username == AdminUsername {
			t.Errorf("Random user returned admin!")
			return
		}
	}

	{
		t.Logf("Testing searching for non-existing UserID")
		var userid UserID
		userid.generate()
		gotuser, err := UserFind(userid)
		if err != nil {
			t.Errorf("UserFind on non-existend userid: %v", err)
			return
		}
		if gotuser != nil {
			t.Errorf("UserFind on non-existent userid: Wanted nil, got %v!", gotuser)
			return
		}

		username := fake.UserName()
		// FIXME: Maybe check to see if the username doesn't exist already
		gotuser, err = UserFindByUsername(username)
		if err != nil {
			t.Errorf("UserFindByUsername on non-existend username %s: %v", username, err)
			return
		}
		if gotuser != nil {
			t.Errorf("UserFindByUsername on non-existent username %s: Wanted nil, got %v!", username, gotuser)
			return
		}
	}

	sort.Slice(users, func(i, j int) bool {
		return users[i].UserID < users[j].UserID
	})

	t.Logf("Testing UserGetAll")
	{
		gotusers, err := UserGetAll()
		if err != nil {
			t.Errorf("Getting a list of all users: %v", err)
			return
		}

		// Number of users should be number of test users + admin account
		if len(gotusers) != testUserCount+1 {
			t.Errorf("Expected %d users, got %d!", testUserCount+1, len(gotusers))
			return
		}

		// gotusers should be listed in UserID order.  Compare the two
		// slices, skipping over the admin user.
		i, j := 0, 0
		for i < testUserCount {
			if gotusers[j].Username == AdminUsername {
				j++
				continue
			}
			if !compareUsers(&users[i], &gotusers[j], t) {
				t.Errorf("UserGetAll mismatch")
				return
			}
			i++
			j++

		}
	}

	t.Logf("Testing UserIterate")
	{
		i := 0
		err = UserIterate(func(u *User) error {
			if u.Username == AdminUsername {
				return nil
			}
			if !compareUsers(&users[i], u, t) {
				return fmt.Errorf("UserIterate mismatch")
			}
			i++
			return nil
		})
		if err != nil {
			t.Errorf("UserIterate error: %v", err)
			return
		}
	}

	t.Logf("Testing UserIterate error reporting")
	{
		i := 0
		err = UserIterate(func(u *User) error {
			if u.Username == AdminUsername {
				return ErrInternal
			}
			if !compareUsers(&users[i], u, t) {
				return fmt.Errorf("UserIterate mismatch")
			}
			i++
			return nil
		})
		if err != ErrInternal {
			t.Errorf("UserIterate error handling: wanted %v, got %v", ErrInternal, err)
			return
		}
	}

	// UserUpdate
	// - Passwords
	//  - "Normal" changing password
	//  - Changing with wrong password
	// - Changing username in field shouldn't actually change username
	// - Invalid userid
	t.Logf("Testing UserUpdate")
	for i := range users {
		users[i].RealName = fake.FullName()
		users[i].Description = fake.Paragraphs()
		// Don't update password
		err := UserUpdate(&users[i], nil, "", "")
		if err != nil {
			t.Errorf("Updating user: %v", err)
			return
		}

		// Check to see that we still get the same result
		gotuser, err := UserFindByUsername(users[i].Username)
		if err != nil {
			t.Errorf("Finding the user we just created by username: %v", err)
			return
		}

		if !compareUsers(&users[i], gotuser, t) {
			t.Errorf("User data mismatch for user uid %s username %s",
				users[i].UserID, users[i].Username)
			return
		}

		if !gotuser.CheckPassword(TestPassword) {
			t.Errorf("Password failed")
			return
		}

		// Try changing the password
		pwd2 := TestPassword + "2"

		err = UserUpdate(&users[i], &users[i], TestPassword, pwd2)
		if err != nil {
			t.Errorf("Changing password: %v", err)
			return
		}

		// Check that the change worked
		gotuser, err = UserFindByUsername(users[i].Username)
		if err != nil {
			t.Errorf("Finding the user we just created by username: %v", err)
			return
		}

		if !compareUsers(&users[i], gotuser, t) {
			t.Errorf("User data mismatch for user uid %s username %s",
				users[i].UserID, users[i].Username)
			return
		}

		if !gotuser.CheckPassword(pwd2) {
			t.Errorf("New password failed")
			return
		}

		// Try using the wrong password
		err = UserUpdate(&users[i], &users[i], TestPassword, pwd2)
		if err == nil {
			t.Errorf("Expected wrong password to fail, but succeeded!")
			return
		}

		// Change it back
		err = UserUpdate(&users[i], &users[i], pwd2, TestPassword)
		if err != nil {
			t.Errorf("Changing password back: %v", err)
			return
		}

		gotuser, err = UserFindByUsername(users[i].Username)
		if err != nil {
			t.Errorf("Finding the user we just created by username: %v", err)
			return
		}

		if !compareUsers(&users[i], gotuser, t) {
			t.Errorf("User data mismatch for user uid %s username %s",
				users[i].UserID, users[i].Username)
			return
		}

		if !gotuser.CheckPassword(TestPassword) {
			t.Errorf("Re-set password failed")
			return
		}

		// Try changing the username, and make sure it didn't actually change
		copy := users[i]
		copy.Username = "invalid"
		err = UserUpdate(&users[i], nil, "", "")
		if err != nil {
			t.Errorf("Updating user: %v", err)
			return
		}

		// Check to see that we still get the same result
		gotuser, err = UserFindByUsername(users[i].Username)
		if err != nil {
			t.Errorf("Finding the user we just created by username: %v", err)
			return
		}

		if !compareUsers(&users[i], gotuser, t) {
			t.Errorf("User data mismatch for user uid %s username %s",
				users[i].UserID, users[i].Username)
			return
		}

		// Try using a bogus UserID and make sure it fails
		copy.UserID = UserID("invalid")
		err = UserUpdate(&copy, nil, "", "")
		if err == nil {
			t.Errorf("Updating user with bad UserID didn't fail!")
			return
		}
	}

	// DeleteUser
	t.Logf("Testing DeleteUser")
	for i := range users {
		err := DeleteUser(users[i].UserID)
		if err != nil {
			t.Errorf("Deleting user %s: %v", users[i].UserID, err)
			return
		}

		gotuser, err := UserFind(users[i].UserID)
		if err != nil {
			t.Errorf("Error getting deleted user: %v", err)
			return
		}
		if gotuser != nil {
			t.Errorf("Getting deleted user: Expected nil, got %v!", gotuser)
			return
		}

		err = DeleteUser(users[i].UserID)
		if err != ErrUserNotFound {
			t.Errorf("Deleting non-existent user: wanted ErrUserNotfound, got %v", err)
			return
		}
	}

	os.RemoveAll(tmpdir)
}
