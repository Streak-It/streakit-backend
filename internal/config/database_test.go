package config

import "testing"

func TestDatabaseDSNFromURL(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{
			"neon url kept as is",
			"postgresql://u:p@ep-x-pooler.us-east-2.aws.neon.tech/neondb?sslmode=require&channel_binding=require",
			"postgresql://u:p@ep-x-pooler.us-east-2.aws.neon.tech/neondb?sslmode=require&channel_binding=require",
		},
		{
			"remote host without sslmode gets require",
			"postgres://u:p@db.example.com/app",
			"postgres://u:p@db.example.com/app?sslmode=require",
		},
		{
			"localhost left alone",
			"postgres://u:p@localhost:5432/app",
			"postgres://u:p@localhost:5432/app",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (DatabaseConfig{URL: tc.in}).DSN(); got != tc.want {
				t.Fatalf("DSN() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValidateDatabaseURL(t *testing.T) {
	if errs := validateDatabaseURL("postgresql://u:p@ep-x.neon.tech/neondb?sslmode=require", true); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	for _, bad := range []string{"mysql://u:p@h/db", "postgres://u:p@h", "postgres:///db"} {
		if errs := validateDatabaseURL(bad, false); len(errs) == 0 {
			t.Errorf("expected error for %q", bad)
		}
	}
}
