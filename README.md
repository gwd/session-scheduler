session-scheduler is a tool for scheduling [Unconference-style](https://en.wikipedia.org/wiki/Unconference) discussion sections to maximize participation.  The basic idea is:

 1. The event creators define the "slots" available for sessions
 2. Members suggest sessions
 3. Attendees express how interested they are in attending various sessions
 4. The scheduler tries to find a schedule that maximizes utility

This is similar to Ubuntu's [Summit scheduler](https://launchpad.net/summit), but:

 1. Lighter-weight; doesn't require a launchpad account, anybody can create sessions, &c
 2. Allows users to express different levels of interest

At the moment, this is a prototype focused on Xen's Developer Summit.

# Build instructions

Clone the repo:

```bash
git clone https://github.com/gwd/session-scheduler
cd session-scheduler
```

Build the binary:

```bash
go build
```

# Starting instructions

Running it is simple.  From the git repo, run:

```bash
./session-scheduler
```

The first time you run `session-scheduler` it will create `data/event.json` to store data about the event.  It will also create an account named `admin`, generate a random password, and print the password to `stdout`.

```bash
./session-scheduler
2020/05/11 15:34:13 Default location to Europe/Berlin
2020/05/11 15:34:13 New user post: 'admin'
2020/05/11 15:34:13 Administrator account: admin DB7Zixb2RYra
2020/05/11 15:34:13 Listening on localhost:22752
```

# Admin mode

Logging in as `admin`, you are in "Admin mode".  The `admin` account may edit users and sessions, but not express interest in sessions.  If you want to experience life as a user, you'll have to create a user account.

The `admin` account has a "Console" page available.  From there you can initiate the session scheduler
and enable test mode, set the verification code, and other admin activities.

# Deployment

To run elsewhere without cloning the entire repo, copy the
`sessions-scheduler` binary, along with the following directories:
`assets` `templates`.  Also create a directory, `data`, for
session-scheduler to store the databases.

# Miscellaneous notes

Registration requires a verification code.  This will be generated randomly the first time session-scheduler
is run.  It can also be set from the admin console.

Discussion proposals refuse duplicate discussions.

Any given user is limited to generating sessions equal to the total number of session slots.
