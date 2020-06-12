module github.com/gwd/session-scheduler

require (
	github.com/corpix/uarand v0.1.0 // indirect
	github.com/hako/durafmt v0.0.0-20190612201238-650ed9f29a84
	github.com/hjson/hjson-go v3.0.1+incompatible
	github.com/icrowley/fake v0.0.0-20180203215853-4178557ae428
	github.com/jmoiron/sqlx v1.2.1-0.20200324155115-ee514944af4b
	github.com/julienschmidt/httprouter v1.2.0
	github.com/mattn/go-sqlite3 v2.0.2+incompatible
	github.com/microcosm-cc/bluemonday v1.0.2
	github.com/sergi/go-diff v1.0.0 // indirect
	github.com/shurcooL/github_flavored_markdown v0.0.0-20181002035957-2122de532470
	github.com/shurcooL/highlight_diff v0.0.0-20181222201841-111da2e7d480 // indirect
	github.com/shurcooL/highlight_go v0.0.0-20181215221002-9d8641ddf2e1 // indirect
	github.com/shurcooL/octicon v0.0.0-20181222203144-9ff1a4cf27f4 // indirect
	github.com/shurcooL/sanitized_anchor_name v1.0.0 // indirect
	github.com/sourcegraph/annotate v0.0.0-20160123013949-f4cad6c6324d // indirect
	github.com/sourcegraph/syntaxhighlight v0.0.0-20170531221838-bd320f5d308e // indirect
	golang.org/x/crypto v0.0.0-20190701094942-4def268fd1a4
)

go 1.13

replace (
	github.com/gwd/session-scheduler/discussions => ./discussions
	github.com/gwd/session-scheduler/id => ./id
	github.com/gwd/session-scheduler/sessions => ./sessions
)
