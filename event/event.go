package event

import (
	"bytes"
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"

	"github.com/gwd/session-scheduler/id"
)

var deepCopyBuffer bytes.Buffer
var deepCopyEncoder = gob.NewEncoder(&deepCopyBuffer)
var deepCopyDecoder = gob.NewDecoder(&deepCopyBuffer)

type EventStore struct {
	filename string
	*sqlx.DB

	Timetable Timetable

	Discussions DiscussionStore
	Locations   LocationStore

	ScheduleV2    *Schedule
	ScheduleSlots int
	LockedSlots

	ScheduleState
}

type EventOptions struct {
	AdminPwd      string
	dbFilename    string
	storeFilename string
}

var event EventStore

const (
	StoreFilename = "data/event.json"
	DbFilename    = "data/event.sqlite"
	AdminUsername = "admin"
	// 3 days * 3 slots per day
	DefaultSlots = 9
)

func (store *EventStore) Init(adminPwd string) {
	store.Discussions.Init()
	store.Timetable.Init()
	store.ScheduleSlots = store.Timetable.GetSlots()

	store.Locations.Init()
	store.ScheduleV2 = nil

	store.LockedSlots = make([]bool, store.ScheduleSlots)

	if adminPwd == "" {
		adminPwd = id.GenerateRawID(12)
	}

	_, err := NewUser(adminPwd, &User{Username: AdminUsername,
		IsAdmin:    true,
		IsVerified: true,
		RealName:   "Xen Schedule Administrator"})
	if err != nil {
		log.Fatalf("Error creating admin user: %v", err)
	}
	log.Printf("Administrator account: admin %s", adminPwd)

	event.Save()
}

// Reset "event" data, without touching users or discussions
func (store *EventStore) ResetEventData() {
	store.Timetable.Init()
	store.ScheduleSlots = store.Timetable.GetSlots()

	store.Locations.Init()
	store.ScheduleV2 = nil
	store.LockedSlots = make([]bool, store.ScheduleSlots)
	store.Discussions.ResetEventData()

	event.Save()
}

func ResetData() {
	event.ResetEventData()
}

// Reset "user" data -- users, discussions, and interest (keeping admin user).
// This also resets the 'event' data, as it won't make much sense anymore with the
// users and discussions gone.
// This should only be done in test mode!
func (store *EventStore) ResetUserData() {
	_, err := event.Exec(`delete from event_users where username != ?`, AdminUsername)
	if err != nil {
		log.Fatalf("Deleting users: %v", err)
	}

	store.Discussions.Init()
	store.ResetEventData()
}

func (store *EventStore) Load(opt EventOptions) error {
	var err error
	event.DB, err = openDb(opt.dbFilename)
	if err != nil {
		return err
	}

	if store.filename == "" {
		store.filename = opt.storeFilename
	}
	contents, err := ioutil.ReadFile(store.filename)

	if err != nil {
		if os.IsNotExist(err) {
			store.Init(opt.AdminPwd)
			return nil
		}
		return err
	}

	// Restoring from an existing file at this point
	err = json.Unmarshal(contents, store)
	if err != nil {
		return err
	}

	if opt.AdminPwd != "" {
		admin, err := UserFindByUsername(AdminUsername)
		if err != nil || admin == nil {
			log.Fatalf("Can't find admin user: %v", err)
		}
		log.Printf("Resetting admin password")
		admin.SetPassword(opt.AdminPwd)
	}

	// Clean up any stale 'running' data
	if event.ScheduleState.IsRunning() {
		event.ScheduleState.SearchFailed()
	}

	// Run timetable placement to update discussion info
	if event.ScheduleV2 != nil {
		event.ScheduleV2.LoadPost()
		event.Timetable.Place(event.ScheduleV2)
	}
	return nil
}

func Load(opt EventOptions) error {
	if opt.storeFilename == "" {
		opt.storeFilename = StoreFilename
	}
	if opt.dbFilename == "" {
		opt.dbFilename = DbFilename
	}
	return event.Load(opt)
}

func (store *EventStore) Save() error {
	contents, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(store.filename, contents, 0660)
}

type LockedSlots []bool

func (ls *LockedSlots) Set(new LockedSlots) {
	*ls = new
	event.Timetable.UpdateIsFinal(new)
	event.Save()
}

func LockedSlotsSet(new LockedSlots) {
	event.LockedSlots.Set(new)
}

func UserFind(userid UserID) (*User, error) {
	var user User
	err := event.Get(&user, `select * from event_users where userid=?`,
		userid)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &user, err
}

// Return nil for user not present
func UserFindByUsername(username string) (*User, error) {
	var user User
	// FIXME: Consider making usernames case-insensitive.  This
	// involves making the whole column case-insensitive (with collate
	// nocase) so that case-insensitive-uniqueness is maintianed; it
	// also means adding unit tests to ensure that case-differeng
	// usernames collide, and that case-differeng usernames are found
	// by the various searches.
	err := event.Get(&user, `select * from event_users where username=?`,
		username)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &user, err
}

func UserFindRandom() (*User, error) {
	var user User
	err := event.Get(&user, `
    select * from event_users
        where username != ?
        order by RANDOM() limit 1`, AdminUsername)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &user, err
}

// Iterate over all users, calling f(u) for each user.
func UserIterate(f func(u *User) error) (err error) {
	rows, err := event.Queryx(`select * from event_users order by userid`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var user User
		if err := rows.StructScan(&user); err != nil {
			return err
		}
		err = f(&user)
		if err != nil {
			return err
		}
	}
	return nil
}

func UserGetAll() (users []User, err error) {
	err = event.Select(&users, `select * from event_users order by userid`)
	return users, err
}

// func (ustore *UserStore) DeepCopy(ucopy *UserStore) (err error) {
// 	if err = deepCopyEncoder.Encode(ustore); err != nil {
// 		return err
// 	}
// 	if err = deepCopyDecoder.Decode(ucopy); err != nil {
// 		return err
// 	}
// 	return nil
// }

func (dstore *DiscussionStore) DeepCopy(dcopy *DiscussionStore) (err error) {
	if err = deepCopyEncoder.Encode(dstore); err != nil {
		return err
	}
	if err = deepCopyDecoder.Decode(dcopy); err != nil {
		return err
	}
	return nil
}

type DiscussionStore map[DiscussionID]*Discussion

func (dstore *DiscussionStore) Init() {
	*dstore = DiscussionStore(make(map[DiscussionID]*Discussion))
}

// Update PossibleSlot size while retaining other information
func (dstore *DiscussionStore) ResetEventData() {
	dstore.Iterate(func(disc *Discussion) error {
		disc.PossibleSlots = MakePossibleSlots(event.ScheduleSlots)
		return nil
	})
}

func (dstore DiscussionStore) Find(id DiscussionID) (*Discussion, error) {
	discussion, exists := (dstore)[id]
	if !exists {
		return nil, nil
	}

	return discussion, nil
}

func (dstore DiscussionStore) Iterate(f func(d *Discussion) error) (err error) {
	for _, disc := range dstore {
		err = f(disc)
		if err != nil {
			return
		}
	}
	return
}

func DiscussionIterate(f func(d *Discussion) error) (err error) {
	return event.Discussions.Iterate(f)
}

func (dstore DiscussionStore) Save(discussion *Discussion) error {
	dstore[discussion.ID] = discussion
	return event.Save()
}

func (dstore DiscussionStore) Delete(did DiscussionID) error {
	delete(dstore, did)
	event.ScheduleState.Modify()
	return event.Save()
}

func (dstore DiscussionStore) GetDidListUser(uid UserID) (list []DiscussionID) {
	dstore.Iterate(func(d *Discussion) error {
		if d.Owner == uid {
			list = append(list, d.ID)
		}
		return nil
	})

	return
}

func ScheduleGetSlots() int {
	return event.ScheduleSlots
}
