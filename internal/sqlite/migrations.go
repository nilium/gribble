package sqlite

var systemPatches = PatchSet{
	// Initialize gribble tables and indices
	StatementPatch("gribble-init", "base-system", 1,
		`CREATE TABLE runners(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token TEXT UNIQUE ON CONFLICT ABORT,
			description TEXT,
			run_untagged BOOLEAN DEFAULT 0,
			locked BOOLEAN DEFAULT 0,
			active BOOLEAN DEFAULT 0,
			max_timeout INTEGER DEFAULT 0,
			deleted BOOLEAN DEFAULT 0,
			created_time REALTIME,
			updated_time REALTIME
		)`,

		`CREATE TABLE runner_locks(
			runner INTEGER,
			project INTEGER,
			PRIMARY KEY(runner, project),
			FOREIGN KEY(runner) REFERENCES runners(id),
			FOREIGN KEY(project) REFERENCES projects(id)
		)`,

		`CREATE TABLE projects(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source TEXT, -- such as 'github'
			source_id NUMERIC,

			name TEXT,
			path TEXT,
			url TEXT,
			clone_url TEXT
		)`,
		`CREATE INDEX projects_by_source ON projects(source, source_id)`,

		`CREATE TABLE tags(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tag TEXT UNIQUE ON CONFLICT ABORT
		)`,

		`CREATE TABLE runner_tags(
			tag INTEGER,
			runner INTEGER,

			PRIMARY KEY(tag, runner),
			FOREIGN KEY(tag) REFERENCES tags(id),
			FOREIGN KEY(runner) REFERENCES runners(id)
		)`,
		`CREATE INDEX runner_tags_by_runner ON runner_tags (runner)`,

		`CREATE TABLE jobs(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			runner INTEGER,
			features INTEGER DEFAULT 0,
			state TEXT DEFAULT 'pending', -- gciwire.JobState
			project INTEGER,
			spec JSON, -- common.JobSpec
			created_time REALTIME,
			finished_time REALTIME,

			FOREIGN KEY(runner) REFERENCES runners(id)
		)`,

		`CREATE TABLE job_depends(
			-- dest is dependent on src
			dest INTEGER,
			src INTEGER CHECK(src < dest),
			fetch_artifacts BOOLEAN DEFAULT 0,

			PRIMARY KEY(src, dest),
			FOREIGN KEY(src) REFERENCES jobs(id),
			FOREIGN KEY(dest) REFERENCES jobs(id)
		)`,
	),
}
