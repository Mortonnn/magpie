package provider

// A key's spend cap: how much an API key may spend, in USD, as magpie's
// own usage prices its calls (sparkling_lemur_62868 on Discord: a team's
// OpenAI key, credits granted to it, that other services spend from too,
// with nothing on OpenAI's side to stop magpie at a share of it). Once
// what magpie sent on the key in the cap's period costs the cap or more,
// routing takes the key for used up — as a usage cap holds an account —
// and goes on to the provider's other keys and its fallbacks; with none,
// the request is answered 429 with why. The key is used again when the
// period renews: the next day or month, or never for a cap in all until
// it is raised or lifted.
//
// What is counted is what the Usage page prices the key's calls at
// (usage.KeySpent), not the vendor's bill: a price magpie doesn't know
// counts nothing, and requests already under way when the cap is reached
// still finish, so the key can go a little past it.

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// SpendCap is a key's spend cap over Period, which is SpendDay, SpendMonth
// or SpendTotal (in all, counted from Since).
//
// The amount is USD: what the cap holds the key at, as magpie's usage
// prices its calls. A cap may also be typed in yuan, the way a key's
// balance is (key_balance_own.go): CNY keeps the amount as typed and Rate
// the dollar's rate in yuan when it was set, while USD stays its value at
// that rate, which is what routing holds the key at whatever the rate does
// afterwards.
type SpendCap struct {
	USD float64 `json:"usd"`
	// CNY is the amount when it was typed in yuan; USD is then unused by a
	// command that sets it (SetKeySpendCap fills USD at Rate), and is the
	// amount's value at Rate once kept
	CNY *float64 `json:"cny,omitempty"`
	// Rate is the yuan a dollar was counted at when the cap was typed in
	// yuan; 0 for one in USD
	Rate   float64 `json:"rate,omitempty"`
	Period string  `json:"period"`
	// Since is when a cap in all was set, what it counts from; kept while
	// the cap stays one in all, its amount changed or not
	Since time.Time `json:"since,omitzero"`
}

// Amount is the cap as typed and its currency, "cny" or "usd".
func (c SpendCap) Amount() (float64, string) {
	if c.CNY != nil {
		return *c.CNY, "cny"
	}
	return c.USD, "usd"
}

// The periods a spend cap counts over: a calendar day or month in local
// time, or in all since it was set.
const (
	SpendDay   = "day"
	SpendMonth = "month"
	SpendTotal = "total"
)

// MaxSpendCap bounds a spend cap, in USD.
const MaxSpendCap = 1_000_000

// Window is the period c counts over at now: when it began and when the
// next begins, zero for a cap in all, which never renews.
func (c SpendCap) Window(now time.Time) (start, reset time.Time) {
	y, m, d := now.Date()
	loc := now.Location()
	switch c.Period {
	case SpendMonth:
		return time.Date(y, m, 1, 0, 0, 0, 0, loc), time.Date(y, m+1, 1, 0, 0, 0, 0, loc)
	case SpendTotal:
		return c.Since, time.Time{}
	}
	return time.Date(y, m, d, 0, 0, 0, 0, loc), time.Date(y, m, d+1, 0, 0, 0, 0, loc)
}

// KeySpendCap is the spend cap of p's key keyID, and whether it has one.
func (p Provider) KeySpendCap(keyID string) (SpendCap, bool) {
	c, ok := p.KeySpendCaps[strings.ToLower(keyID)]
	return c, ok && c.USD > 0
}

// SetKeySpendCap gives the key ref (its id or name) of the provider id
// the spend cap c, nil to take its cap off. A cap in all keeps the Since
// it had while it stays one; else it counts from now.
func SetKeySpendCap(id, ref string, c *SpendCap) error {
	if c != nil {
		if err := prepareSpendCap(c); err != nil {
			return err
		}
	}
	p, err := Find(id)
	if err != nil {
		return err
	}
	if p.Account != nil {
		return fmt.Errorf("%s is a subscription: a spend cap is for an API key (an account's cap is a share of its usage windows, magpie provider account-cap)", p.Name)
	}
	r, ok := p.accountRef(ref)
	if !ok {
		return fmt.Errorf("%s has no key %q", p.Name, ref)
	}
	k := strings.ToLower(r)
	m := map[string]SpendCap{}
	for id, v := range p.KeySpendCaps {
		m[id] = v
	}
	if c == nil {
		if _, had := m[k]; !had {
			return nil
		}
		delete(m, k)
	} else {
		v := *c
		v.Since = time.Time{}
		if v.Period == SpendTotal {
			v.Since = time.Now().UTC().Truncate(time.Second)
			if was, had := m[k]; had && was.Period == SpendTotal && !was.Since.IsZero() {
				v.Since = was.Since
			}
		}
		m[k] = v
	}
	p.KeySpendCaps = m
	return Save(*p)
}

// prepareSpendCap fills in what a cap typed by a caller leaves out: one in
// yuan has USD worked out at the dollar's rate in yuan magpie last learned
// (never a guess), and Rate kept with it so what it shows stays what it is
// held at. A cap already in USD is only checked.
func prepareSpendCap(c *SpendCap) error {
	if c.CNY != nil {
		if c.USD != 0 {
			return fmt.Errorf("a spend cap is in USD or in yuan, not both")
		}
		rate, known := cnyPerUSD()
		if !known || rate <= 0 {
			return fmt.Errorf("a spend cap in yuan needs the dollar's rate in yuan, which magpie hasn't learned yet; try again in a moment, or set the cap in USD")
		}
		c.Rate = rate
		c.USD = *c.CNY / rate
	}
	return checkSpendCap(*c)
}

// checkSpendCap says what is wrong with a cap as it is kept, nil when
// nothing is: the period and an amount above 0 and up to MaxSpendCap, in
// its own currency, with a cap in yuan also carrying the rate it was set
// at and the USD it worked out to.
func checkSpendCap(c SpendCap) error {
	switch c.Period {
	case SpendDay, SpendMonth, SpendTotal:
	default:
		return fmt.Errorf("a spend cap is per %s, per %s or in all (%s), not %q", SpendDay, SpendMonth, SpendTotal, c.Period)
	}
	if c.CNY != nil {
		if !(*c.CNY > 0 && *c.CNY <= MaxSpendCap) || math.IsNaN(*c.CNY) {
			return fmt.Errorf("a spend cap is an amount in yuan above 0 and up to %d, not %v", MaxSpendCap, *c.CNY)
		}
		if !(c.Rate > 0) || math.IsNaN(c.Rate) {
			return fmt.Errorf("a spend cap in yuan needs the dollar's rate in yuan it was set at, not %v", c.Rate)
		}
		if !(c.USD > 0 && c.USD <= MaxSpendCap) || math.IsNaN(c.USD) {
			return fmt.Errorf("a spend cap in yuan works out to %v USD, not above 0 and up to %d", c.USD, MaxSpendCap)
		}
		return nil
	}
	if !(c.USD > 0 && c.USD <= MaxSpendCap) || math.IsNaN(c.USD) {
		return fmt.Errorf("a spend cap is an amount in USD above 0 and up to %d, not %v", MaxSpendCap, c.USD)
	}
	return nil
}

// checkSpendCapAmount says what is wrong with the period and the amount c
// types, before a cap in yuan is worked out in USD.
func checkSpendCapAmount(c SpendCap) error {
	switch c.Period {
	case SpendDay, SpendMonth, SpendTotal:
	default:
		return fmt.Errorf("a spend cap is per %s, per %s or in all (%s), not %q", SpendDay, SpendMonth, SpendTotal, c.Period)
	}
	if c.CNY != nil {
		if c.USD != 0 {
			return fmt.Errorf("a spend cap is in USD or in yuan, not both")
		}
		v := *c.CNY
		if !(v > 0 && v <= MaxSpendCap) || math.IsNaN(v) {
			return fmt.Errorf("a spend cap is an amount in yuan above 0 and up to %d, not %v", MaxSpendCap, v)
		}
		return nil
	}
	if !(c.USD > 0 && c.USD <= MaxSpendCap) || math.IsNaN(c.USD) {
		return fmt.Errorf("a spend cap is an amount in USD above 0 and up to %d, not %v", MaxSpendCap, c.USD)
	}
	return nil
}

// ParseSpendCap reads a spend cap as the CLI takes it: an amount in USD
// ("20", "$20", "12.5") or in yuan ("¥140", "140cny", "140 rmb", "140元")
// and its period (day, month or total, per day by default); off for none
// (nil). A cap in yuan is worked out in USD when it is saved, at the rate
// magpie last learned.
func ParseSpendCap(amount, period string) (*SpendCap, error) {
	a := strings.ToLower(strings.TrimSpace(amount))
	switch a {
	case "off", "none", "no", "-":
		return nil, nil
	}
	cny := false
	for _, s := range []string{"¥", "￥", "cny", "rmb"} {
		if strings.HasPrefix(a, s) {
			a, cny = strings.TrimPrefix(a, s), true
		}
	}
	for _, s := range []string{"元", "cny", "rmb", "yuan"} {
		if strings.HasSuffix(a, s) {
			a, cny = strings.TrimSuffix(a, s), true
		}
	}
	if !cny {
		a = strings.TrimSuffix(strings.TrimPrefix(a, "$"), "usd")
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(a), ",", ""), 64)
	if err != nil {
		return nil, fmt.Errorf("a spend cap is an amount in USD (20, 12.5), in yuan (¥140) or off, not %q", amount)
	}
	per := strings.ToLower(strings.TrimSpace(period))
	switch per {
	case "", "daily", "a-day", "per-day":
		per = SpendDay
	case "monthly", "a-month", "per-month":
		per = SpendMonth
	case "all", "in-all", "ever":
		per = SpendTotal
	}
	c := SpendCap{USD: v, Period: per}
	if cny {
		c = SpendCap{CNY: &v, Period: per}
	}
	if err := checkSpendCapAmount(c); err != nil {
		return nil, err
	}
	return &c, nil
}

// normalKeySpendCaps is m as it is kept: key ids in lower case, caps that
// say nothing left out, a cap in all with no Since counting from the
// epoch (one written by hand); nil for none.
func normalKeySpendCaps(m map[string]SpendCap) map[string]SpendCap {
	var out map[string]SpendCap
	for k, c := range m {
		k = strings.ToLower(strings.TrimSpace(k))
		if k == "" || checkSpendCap(c) != nil {
			continue
		}
		if c.Period != SpendTotal {
			c.Since = time.Time{}
		}
		if out == nil {
			out = map[string]SpendCap{}
		}
		out[k] = c
	}
	return out
}
