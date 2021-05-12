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

The first time you run `session-scheduler` it will create a number of
databases in `data/`:
 * `serverconfig.sqlite` to store data about configuration: default
  port number, admin password, and so on, so that you don't have to
  enter these in every time
 * `event.sqlite` to store data about the event itself: timetables,
locations, users, discussions, interest
 * `sessions.sqlite` to store data about active user sessions.  (This
   means you can restart your server and logged-in users are still
   logged in.)

It will also create an account named `admin`, generate a random
password, and print the password to `stdout`; and if the port number
hasn't been specified, it will generate a random port number and bind
to it.

```bash
./session-scheduler
2020/05/11 15:34:13 Default location to Europe/Berlin
2020/05/11 15:34:13 New user post: 'admin'
2020/05/11 15:34:13 Administrator account: admin DB7Zixb2RYra
2020/05/11 15:34:13 Listening on localhost:22752
```

# Admin mode

Logging in as `admin`, you are in "Admin mode".  The `admin` account
may edit users and sessions, but not express interest in sessions.  If
you want to experience life as a user, you'll have to create a user
account.

The `admin` account has a "Console" page available.  From there you
can initiate the session scheduler and enable test mode, set the
verification code, and other admin activities.

# Deployment

To run elsewhere without cloning the entire repo, copy the
`sessions-scheduler` binary, along with the following directories:
`assets` `templates`.  Also create a directory, `data`, for
session-scheduler to store the databases.

`deploy.sh` has been created to do this automatically; it's hard-coded
with our server, user, and directory, so modify it according to your
use case.

There is a `-serverlock` method you can use to handle potential
duplicate copies of the same server; `quit` will exit if another
instance is running, `error` will exit with an error, and `wait` will
wait for the other instance to quit.

Current recommended practice is to start the server in a `tmux`
session, like so:

```
./session-scheduler -servelock quit >> sessions-scheduler.log &
tail -f sessions-scheduler.log
```

And then add a crontab like the following:

```
@reboot /home/xensched/session-scheduler/session-scheduler -servelock quit >> /home/xensched/session-scheduler/session-scheduler.log
```

# Backups

The most robust way to create automatic backups is to use the
`sqlite3` tool with the `VACUUM` command, and put it in your crontab.
Below is an example crontab entry:

```
0 4 * * * sqlite3 session-scheduler/data/event.db "vacuum into \"backup/event-$(date -Iminutes).sqlite\""
```

This is also handy for getting a copy of the "live" database to do
debugging.

# Miscellaneous notes

Registration requires a verification code.  This will be generated randomly the first time session-scheduler
is run.  It can also be set from the admin console.

Discussion proposals refuse discussions with duplicate titles.

Any given user is limited to generating sessions equal to the total number of session slots.
