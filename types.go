package main

// types.go - built-in types for schema rules: type=date, type=x11color...
//
// Each built-in type checks a value and returns it in its standard
// spelling, the way bool and enum already do: input is forgiving, output
// is standard. A type also says in words what it accepts (its hint, used
// where a rule has no format of its own), and may offer choices for Tab.
//
// Everything about built-in types lives here and in x11colors.go. The
// rest of the program reaches them only through rule.bt, so removing
// these two files and the few "r.bt != nil" lines in schema.go takes the
// feature out again.

import (
	"fmt"
	"net/netip"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // time zone names work on Windows too
)

// builtin is one built-in type.
type builtin struct {
	name    string
	hint    string                       // what it accepts, in words
	check   func(string) (string, error) // the standard spelling, or why not
	choices []string                     // what Tab offers; nil for none
}

var builtins = map[string]*builtin{}

func addBuiltin(b *builtin) { builtins[b.name] = b }

// builtinType finds a built-in type by name; nil if there is none.
func builtinType(name string) *builtin { return builtins[name] }

// builtinNames lists the built-in types, sorted, for error messages.
func builtinNames() []string {
	var out []string
	for n := range builtins {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

var (
	reTime24   = regexp.MustCompile(`^([01]?[0-9]|2[0-3]):([0-5][0-9])$`)
	reTime12   = regexp.MustCompile(`(?i)^(0?[1-9]|1[0-2]):([0-5][0-9]) ?([ap])\.?m\.?$`)
	reDate     = regexp.MustCompile(`^([0-9]{4})-([0-9]{1,2})-([0-9]{1,2})$`)
	reDuration = regexp.MustCompile(`^([0-9]+(ms|s|m|h|d))+$`)
	reSize     = regexp.MustCompile(`(?i)^([0-9]+) ?(b|kb|mb|gb|tb)$`)
	reHexColor = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
	reEmail    = regexp.MustCompile(`^[^@\s]+@([^@\s]+\.[^@\s]+)$`)
	reHostName = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?(\.[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?)*$`)
)

func init() {
	addBuiltin(&builtin{
		name: "x11color",
		hint: "an X11 color name, e.g. SteelBlue; Tab lists them",
		check: func(v string) (string, error) {
			if std, ok := x11ByKey[colorKey(v)]; ok {
				return std, nil
			}
			if near := nearestColor(v); near != "" {
				return "", fmt.Errorf("%q isn't an X11 color name; did you mean %s?", v, near)
			}
			return "", fmt.Errorf("%q isn't an X11 color name", v)
		},
		choices: x11Colors,
	})
	addBuiltin(&builtin{
		name: "hexcolor",
		hint: "#RRGGBB, e.g. #4682B4",
		check: func(v string) (string, error) {
			if !reHexColor.MatchString(v) {
				return "", fmt.Errorf("%q isn't a color in the form #RRGGBB", v)
			}
			return strings.ToUpper(v), nil
		},
	})
	addBuiltin(&builtin{
		name: "date",
		hint: "YYYY-MM-DD, e.g. 2026-11-05",
		check: func(v string) (string, error) {
			m := reDate.FindStringSubmatch(v)
			if m == nil {
				return "", fmt.Errorf("%q isn't a date in the form YYYY-MM-DD", v)
			}
			y, _ := strconv.Atoi(m[1])
			mo, _ := strconv.Atoi(m[2])
			d, _ := strconv.Atoi(m[3])
			t := time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.UTC)
			if mo < 1 || mo > 12 || t.Day() != d || int(t.Month()) != mo {
				return "", fmt.Errorf("%s isn't a real date", v)
			}
			return t.Format("2006-01-02"), nil
		},
	})
	addBuiltin(&builtin{
		name: "time",
		hint: "HH:MM, 24-hour, e.g. 07:30",
		check: func(v string) (string, error) {
			m := reTime24.FindStringSubmatch(v)
			if m == nil {
				return "", fmt.Errorf("%q isn't a 24-hour time (HH:MM)", v)
			}
			h, _ := strconv.Atoi(m[1])
			return fmt.Sprintf("%02d:%s", h, m[2]), nil
		},
	})
	addBuiltin(&builtin{
		name: "time12",
		hint: "12-hour time, e.g. 7:30 PM",
		check: func(v string) (string, error) {
			m := reTime12.FindStringSubmatch(strings.TrimSpace(v))
			if m == nil {
				return "", fmt.Errorf("%q isn't a 12-hour time (e.g. 7:30 PM)", v)
			}
			h, _ := strconv.Atoi(m[1])
			return fmt.Sprintf("%d:%s %sM", h, m[2], strings.ToUpper(m[3])), nil
		},
	})
	addBuiltin(&builtin{
		name: "duration",
		hint: "a duration such as 90s, 15m or 1h30m",
		check: func(v string) (string, error) {
			s := strings.ToLower(strings.ReplaceAll(v, " ", ""))
			if !reDuration.MatchString(s) {
				return "", fmt.Errorf("%q isn't a duration (e.g. 90s, 15m, 1h30m; units ms, s, m, h, d)", v)
			}
			return s, nil
		},
	})
	addBuiltin(&builtin{
		name: "size",
		hint: "a size such as 512MB or 2GB",
		check: func(v string) (string, error) {
			m := reSize.FindStringSubmatch(strings.TrimSpace(v))
			if m == nil {
				return "", fmt.Errorf("%q isn't a size (e.g. 512MB, 2GB; units B, KB, MB, GB, TB)", v)
			}
			n, _ := strconv.ParseUint(m[1], 10, 64)
			return fmt.Sprintf("%d%s", n, strings.ToUpper(m[2])), nil
		},
	})
	addBuiltin(&builtin{
		name: "ipv4",
		hint: "an IPv4 address, e.g. 192.168.1.20",
		check: func(v string) (string, error) {
			a, err := netip.ParseAddr(v)
			if err != nil || !a.Is4() {
				return "", fmt.Errorf("%q isn't an IPv4 address", v)
			}
			return a.String(), nil
		},
	})
	addBuiltin(&builtin{
		name: "ip",
		hint: "an IP address, e.g. 192.168.1.20 or fe80::1",
		check: func(v string) (string, error) {
			a, err := netip.ParseAddr(v)
			if err != nil || a.Zone() != "" {
				return "", fmt.Errorf("%q isn't an IP address", v)
			}
			return a.String(), nil
		},
	})
	addBuiltin(&builtin{
		name: "host",
		hint: "a host name or IP address, e.g. signage-07.example.com",
		check: func(v string) (string, error) {
			if a, err := netip.ParseAddr(v); err == nil && a.Zone() == "" {
				return a.String(), nil
			}
			if len(v) > 253 || !reHostName.MatchString(v) {
				return "", fmt.Errorf("%q isn't a host name or IP address", v)
			}
			return strings.ToLower(v), nil
		},
	})
	addBuiltin(&builtin{
		name: "url",
		hint: "a web address starting with http:// or https://",
		check: func(v string) (string, error) {
			u, err := url.Parse(v)
			if err != nil || (strings.ToLower(u.Scheme) != "http" && strings.ToLower(u.Scheme) != "https") || u.Host == "" {
				return "", fmt.Errorf("%q isn't a web address starting with http:// or https://", v)
			}
			u.Scheme = strings.ToLower(u.Scheme)
			u.Host = strings.ToLower(u.Host)
			return u.String(), nil
		},
	})
	addBuiltin(&builtin{
		name: "email",
		hint: "an email address, e.g. it@example.com",
		check: func(v string) (string, error) {
			m := reEmail.FindStringSubmatch(v)
			if m == nil {
				return "", fmt.Errorf("%q isn't an email address", v)
			}
			at := len(v) - len(m[1])
			return v[:at] + strings.ToLower(m[1]), nil // the domain ignores case
		},
	})
	addBuiltin(&builtin{
		name: "timezone",
		hint: "a time zone name, e.g. America/New_York",
		check: func(v string) (string, error) {
			if v == "Local" || v == "" {
				return "", fmt.Errorf("%q isn't a time zone name (e.g. America/New_York, UTC)", v)
			}
			if _, err := time.LoadLocation(v); err != nil {
				return "", fmt.Errorf("%q isn't a time zone name (e.g. America/New_York, UTC)", v)
			}
			return v, nil
		},
	})
	addBuiltin(&builtin{
		name: "port",
		hint: "a port number, 1 to 65535",
		check: func(v string) (string, error) {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 || n > 65535 {
				return "", fmt.Errorf("%q isn't a port number (1 to 65535)", v)
			}
			return strconv.Itoa(n), nil
		},
	})
}

// colorKey is how color names are compared: case and spaces ignored, so
// "steel blue" and "STEELBLUE" are both SteelBlue.
func colorKey(v string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(v), " ", ""))
}

var x11ByKey = func() map[string]string {
	m := map[string]string{}
	for _, c := range x11Colors {
		m[colorKey(c)] = c
	}
	return m
}()

// nearestColor suggests the closest color name to a near miss, or "" when
// nothing is close.
func nearestColor(v string) string {
	key := colorKey(v)
	if key == "" {
		return ""
	}
	// Allow about one slip per four letters, between one and three.
	limit := min(max(len(key)/4, 1), 3)
	best, bestDist := "", limit+1
	for _, c := range x11Colors {
		if d := editDistance(key, colorKey(c)); d < bestDist {
			best, bestDist = c, d
		}
	}
	return best
}

// editDistance counts the single-letter slips between a and b: a letter
// added, dropped or changed, or two neighbours swapped.
func editDistance(a, b string) int {
	rows := make([][]int, len(a)+1)
	for i := range rows {
		rows[i] = make([]int, len(b)+1)
		rows[i][0] = i
	}
	for j := range rows[0] {
		rows[0][j] = j
	}
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			d := min(rows[i-1][j]+1, rows[i][j-1]+1, rows[i-1][j-1]+cost)
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				d = min(d, rows[i-2][j-2]+1)
			}
			rows[i][j] = d
		}
	}
	return rows[len(a)][len(b)]
}
