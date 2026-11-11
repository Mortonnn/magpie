// A key's spend cap (sparkling_lemur_62868 on Discord: a team's OpenAI
// key, with credits other services spend from too): how much magpie may
// spend on it, in USD at the Usage page's prices, a day, a month or in
// all. Past it the gateway passes the key over, as a usage cap holds an
// account (provider/key_spend.go). It may be typed in yuan like a key's
// balance (keybalance.js): the amount and a currency button beside the
// field, the cap held at the rate magpie last learned. app.js imports this
// when the provider editor draws a key's row (keySpend), so it weighs
// nothing on the core.
const spendMoney = (v, cny) => (v < 0 ? "-" : "") + (cny ? "¥" : "$") + Math.abs(v).toFixed(2);
const SPEND_PERIODS = { day: "a day", month: "a month", total: "in all" };
function keySpendPill(p, k) {
  const sc = k.spend, name = k.name || k.masked, cny = sc?.cny != null;
  const pill = el("button", "aspend" + (sc ? " set" : "") + (sc?.held ? " at-cap" : ""));
  pill.type = "button";
  // spent is USD at the Usage page's prices; a cap typed in yuan tells it in
  // yuan at the rate the cap was set at
  const v = sc && {
    spent: spendMoney(cny ? (sc.spent || 0) * (sc.rate || 0) : sc.spent || 0, cny),
    cap: spendMoney(cny ? sc.cny : sc.usd, cny),
  };
  pill.textContent = !sc ? t("No spend cap") : t(sc.period === "month" ? "{spent} of {cap} this month" : sc.period === "total" ? "{spent} of {cap} in all" : "{spent} of {cap} today", v);
  pill.title = sc ? t("Stops at {cap} {period}, counted at the prices the Usage page shows for the calls magpie sends on this key. Past it, magpie goes on to the provider's other keys and its fallbacks. Requests under way still finish, so it can go a little past the cap, and a model with no known price counts nothing. Click to change", { cap: v.cap, period: t(SPEND_PERIODS[sc.period] || "a day") })
    : t("No spend cap: magpie sends this key requests whatever they cost. Click to set how much it may spend, a day, a month or in all; past it, magpie goes on to the provider's other keys and its fallbacks");
  pill.setAttribute("aria-haspopup", "menu");
  pill.setAttribute("aria-expanded", "false");
  // a bare number is in the currency the button beside the field shows: the
  // cap's own, else Settings' one, until the user picks the other there; a
  // sign typed in the field says its currency, and the button follows
  let unit = sc ? (cny ? "cny" : "usd") : currency === "cny" ? "cny" : "usd";
  const amount = (s) => {
    let a = s.trim().toLowerCase(), c = unit;
    const yuan = /^(¥|￥|cny|rmb)\s*|\s*(元|cny|rmb|yuan)$/g, usd = /^\$\s*|\s*usd$/g;
    if (yuan.test(a)) { a = a.replace(yuan, ""); c = "cny"; }
    else if (usd.test(a)) { a = a.replace(usd, ""); c = "usd"; }
    a = a.replace(/,/g, "").trim();
    const n = Number(a);
    if (!(a && n > 0 && n <= 1e6)) return null;
    unit = c;
    return n;
  };
  const set = (n, per) => {
    const spend = n ? (unit === "cny" ? { cny: n, period: per } : { usd: n, period: per }) : null;
    accountAction("provider/keyspendcap", { id: p.id, account: k.id, spend },
      !n ? t("{who} has no spend cap", { who: name })
        : t(per === "month" ? "{who} stops at {usd} a month" : per === "total" ? "{who} stops at {usd} in all" : "{who} stops at {usd} a day", { who: name, usd: spendMoney(n, unit === "cny") }));
  };
  // the amount typed in place, with the currency button beside it
  const type = (per) => {
    const at = pill.parentNode;
    pillNumber(pill, sc ? String(cny ? sc.cny : sc.usd) : "", "", "aspend-in", t(unit === "cny" ? "Spend cap, in CNY" : "Spend cap, in USD"), amount, t("A spend cap is an amount in USD or in yuan (¥) above 0, up to 1,000,000"), sc ? (cny ? sc.cny : sc.usd) : null, (n) => set(n, per));
    const i = at?.querySelector(":scope > .aspend-in");
    if (!i) return;
    const cur = el("button", "aspend-cur"), save = i.onblur;
    cur.type = "button";
    cur.setAttribute("aria-haspopup", "menu");
    cur.setAttribute("aria-expanded", "false");
    let picking = false;
    const show = () => {
      cur.replaceChildren(unit.toUpperCase(), svg(CHEV, 12, 1.6));
      cur.setAttribute("aria-label", t("Currency"));
      i.setAttribute("aria-label", t(unit === "cny" ? "Spend cap, in CNY" : "Spend cap, in USD"));
    };
    i.oninput = () => { if (amount(i.value) != null) show(); };
    // the field saves as it is left, but not for its currency's menu
    i.onblur = (e) => { if (!picking && e.relatedTarget !== cur) save(); };
    cur.onblur = (e) => { if (!picking && e.relatedTarget !== i) save(); };
    cur.onmousedown = (e) => e.preventDefault();
    cur.onkeydown = (e) => e.stopPropagation();
    cur.onclick = (e) => {
      e.stopPropagation();
      if (cur.classList.contains("open")) return closeProtoMenu();
      picking = true;
      openProtoMenu(cur, CURRENCIES.map(([v, name]) => ({ v, name, literalName: true, note: "" })), unit, (v) => {
        unit = v;
        i.value = i.value.trim().replace(/^(\$|¥|￥|cny|rmb|usd)\s*|\s*(元|cny|rmb|yuan|usd)$/gi, "");
        show();
      }, "Currency", "sess-menu");
      protoMenu.done = () => { picking = false; i.focus(NO_SCROLL); };
    };
    show();
    i.after(cur);
  };
  pill.onclick = (e) => {
    e.stopPropagation();
    if (pill.classList.contains("open")) return closeProtoMenu();
    const opts = [
      { v: "off", name: "No spend cap", note: "Spend as much as the key has" },
      { v: "day", name: "Per day…", note: "Counts again each day" },
      { v: "month", name: "Per month…", note: "Counts again each month" },
      { v: "total", name: "In all…", note: "Counted from when it is set" },
    ];
    openProtoMenu(pill, opts, sc?.period || "off", (per) => {
      if (per === "off") return sc && set(0);
      type(per);
    }, "Spend cap", "sess-menu");
  };
  return pill;
}
// keySpendLine: a key held at its spend cap, on a line under its row
function keySpendLine(k) {
  const sc = k.spend;
  if (!sc?.held) return null;
  const s = el("span", "key-rest", sc.reset ? t("At its spend cap · back {in}", { in: untilText(new Date(sc.reset)) }) : t("At its spend cap · until it is raised"));
  s.title = t("This key has spent its cap, so the gateway passes it over and tries the next key or a fallback. With none, requests are refused with a spend-cap error")
    + (sc.reset ? "\n" + resetText(new Date(sc.reset), new Date(sc.reset).toLocaleString(intlLang())) : "");
  const line = el("div", "key-rest-line key-spend-line");
  line.append(s);
  return line;
}

// keySpendFill puts key k's spend cap pill in its row, before the gap, and
// its held line under the row's other lines
window.keySpendFill = function (row, p, k) {
  // and its typed balance, in keybalance.js, for a provider that takes one
  if (window.keyBalanceFill) keyBalanceFill(row, p, k);
  else if (p.takesOwnBalance) import("./keybalance.js").then(() => keyBalanceFill(row, p, k), () => {});
  row.querySelector(":scope > .aspend")?.remove();
  row.querySelector(":scope > .key-spend-line")?.remove();
  const pill = keySpendPill(p, k), gap = row.querySelector(":scope > .grow");
  gap ? gap.before(pill) : row.append(pill);
  const line = keySpendLine(k);
  if (!line) return;
  const at = row.querySelector(":scope > .key-rest-line") || row.querySelector(":scope > .key-acts");
  at ? at.after(line) : row.append(line);
  row.classList.add("resting");
};
