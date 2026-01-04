package detector

import "regexp"

// TimestampFormat represents a known timestamp format for detection.
type TimestampFormat struct {
	Name       string         // Human-readable name
	Pattern    *regexp.Regexp // Compiled regex (set during init)
	PatternStr string         // Pattern string for config output
	Layout     string         // Go time layout for parsing
	Examples   []string       // Example timestamps
	Ambiguous  bool           // True if format has date ordering ambiguity (MM/DD vs DD/MM)
}

// DefaultFormats returns the built-in timestamp formats to detect.
// Formats are ordered roughly by specificity (more specific patterns first).
func DefaultFormats() []*TimestampFormat {
	formats := []*TimestampFormat{
		// ISO 8601 with timezone offset
		{
			Name:       "ISO 8601 with timezone",
			PatternStr: `^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[+-]\d{2}:\d{2})`,
			Layout:     "2006-01-02T15:04:05-07:00",
			Examples:   []string{"2024-01-15T10:30:00+00:00", "2024-01-15T10:30:00-05:00"},
		},
		// ISO 8601 with Z (UTC)
		{
			Name:       "ISO 8601 with Z (UTC)",
			PatternStr: `^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z)`,
			Layout:     "2006-01-02T15:04:05Z",
			Examples:   []string{"2024-01-15T10:30:00Z"},
		},
		// ISO 8601 with milliseconds and timezone
		{
			Name:       "ISO 8601 with milliseconds and timezone",
			PatternStr: `^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}[+-]\d{2}:\d{2})`,
			Layout:     "2006-01-02T15:04:05.000-07:00",
			Examples:   []string{"2024-01-15T10:30:00.123+00:00"},
		},
		// ISO 8601 with milliseconds and Z
		{
			Name:       "ISO 8601 with milliseconds and Z",
			PatternStr: `^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z)`,
			Layout:     "2006-01-02T15:04:05.000Z",
			Examples:   []string{"2024-01-15T10:30:00.123Z"},
		},
		// ISO 8601 with milliseconds (no timezone)
		{
			Name:       "ISO 8601 with milliseconds",
			PatternStr: `^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3})`,
			Layout:     "2006-01-02T15:04:05.000",
			Examples:   []string{"2024-01-15T10:30:00.123"},
		},
		// ISO 8601 basic (with T separator)
		{
			Name:       "ISO 8601",
			PatternStr: `^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})`,
			Layout:     "2006-01-02T15:04:05",
			Examples:   []string{"2024-01-15T10:30:00"},
		},
		// Bracketed datetime (NegaLog default)
		{
			Name:       "Bracketed datetime",
			PatternStr: `^\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\]`,
			Layout:     "2006-01-02 15:04:05",
			Examples:   []string{"[2024-01-15 10:30:00]"},
		},
		// Syslog with year
		{
			Name:       "Syslog with year",
			PatternStr: `^(\w{3}\s+\d{1,2}\s+\d{4}\s+\d{2}:\d{2}:\d{2})`,
			Layout:     "Jan 2 2006 15:04:05",
			Examples:   []string{"Jun 14 2024 15:16:01"},
		},
		// Syslog BSD format (no year)
		{
			Name:       "Syslog (BSD)",
			PatternStr: `^(\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})`,
			Layout:     "Jan 2 15:04:05",
			Examples:   []string{"Jun 14 15:16:01", "Jan  5 09:30:00"},
		},
		// Apache/NGINX common log format
		{
			Name:       "Apache/NGINX CLF",
			PatternStr: `\[(\d{2}/\w{3}/\d{4}:\d{2}:\d{2}:\d{2}\s+[+-]\d{4})\]`,
			Layout:     "02/Jan/2006:15:04:05 -0700",
			Examples:   []string{"[15/Jun/2024:10:30:00 +0000]"},
		},
		// Apache error log format [Day Mon DD HH:MM:SS YYYY]
		{
			Name:       "Apache error log",
			PatternStr: `^\[(\w{3} \w{3} \d{2} \d{2}:\d{2}:\d{2} \d{4})\]`,
			Layout:     "Mon Jan 02 15:04:05 2006",
			Examples:   []string{"[Sun Dec 04 04:47:44 2005]"},
		},
		// Spark/Hadoop short date YY/MM/DD HH:MM:SS
		{
			Name:       "Spark/Hadoop short date",
			PatternStr: `^(\d{2}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})`,
			Layout:     "06/01/02 15:04:05",
			Examples:   []string{"17/06/09 20:10:40"},
		},
		// HDFS compact format YYMMDD HHMMSS
		{
			Name:       "HDFS compact",
			PatternStr: `^(\d{6} \d{6})`,
			Layout:     "060102 150405",
			Examples:   []string{"081109 203615"},
		},
		// Python logging default (comma for milliseconds)
		{
			Name:       "Python logging",
			PatternStr: `^(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2},\d{3})`,
			Layout:     "2006-01-02 15:04:05,000",
			Examples:   []string{"2024-01-15 10:30:00,123"},
		},
		// Log4j / Java logging (period for milliseconds)
		{
			Name:       "Log4j/Java logging",
			PatternStr: `^(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}\.\d{3})`,
			Layout:     "2006-01-02 15:04:05.000",
			Examples:   []string{"2024-01-15 10:30:00.123"},
		},
		// Space-separated datetime (no brackets)
		{
			Name:       "Datetime (space-separated)",
			PatternStr: `^(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2})`,
			Layout:     "2006-01-02 15:04:05",
			Examples:   []string{"2024-01-15 10:30:00"},
		},
		// Kubernetes/Docker JSON timestamp
		{
			Name:       "Kubernetes JSON timestamp",
			PatternStr: `"time":"(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+Z)"`,
			Layout:     "2006-01-02T15:04:05.000000000Z",
			Examples:   []string{`"time":"2024-01-15T10:30:00.123456789Z"`},
		},
		// Unix timestamp (seconds) - at start of line
		{
			Name:       "Unix timestamp (seconds)",
			PatternStr: `^(\d{10})(?:\s|$|\])`,
			Layout:     "UNIX_SECONDS",
			Examples:   []string{"1705315800"},
		},
		// Unix timestamp (milliseconds) - at start of line
		{
			Name:       "Unix timestamp (milliseconds)",
			PatternStr: `^(\d{13})(?:\s|$|\])`,
			Layout:     "UNIX_MILLIS",
			Examples:   []string{"1705315800000"},
		},
		// US date format MM/DD/YYYY (ambiguous)
		{
			Name:       "US date format (MM/DD/YYYY)",
			PatternStr: `^(\d{2}/\d{2}/\d{4}\s+\d{2}:\d{2}:\d{2})`,
			Layout:     "01/02/2006 15:04:05",
			Examples:   []string{"01/15/2024 10:30:00"},
			Ambiguous:  true,
		},
	}

	// Compile all patterns
	for _, f := range formats {
		f.Pattern = regexp.MustCompile(f.PatternStr)
	}

	return formats
}
