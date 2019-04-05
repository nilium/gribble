package sqlite

var systemPatches = PatchSet{
	StatementPatch("gribble/init", "base-system", 1,
		`CREATE TABLE runners (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token TEXT UNIQUE ON CONFLICT ABORT,
			description TEXT,
			run_untagged BOOLEAN DEFAULT 0,
			locked BOOLEAN DEFAULT 0,
			active BOOLEAN DEFAULT 0,
			max_timeout INT DEFAULT 0,
			deleted BOOLEAN DEFAULT 0
		)`,
		`CREATE TABLE tags (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tag TEXT UNIQUE ON CONFLICT ABORT
		)`,
		`CREATE TABLE runner_tags (
			tag INTEGER,
			runner INTEGER,
			PRIMARY KEY (tag, runner)
		)`,
		`CREATE INDEX runner_tags_by_runner ON runner_tags (runner)`,
		`CREATE TABLE jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT
		)`,
	),
}
