import { AUTHOR, CHANGELOG_URL, OPERATOR, SOURCE_URL } from "./maker";
import { Link } from "./router";
import { pathFor, type RouteId } from "./routes";
import { useLocale, useT } from "../i18n/i18n";

const LINKS: [RouteId | { url: string }, string][] = [
  ["howToPlay", "site.header.howToPlay"],
  ["worlds", "site.header.worlds"],
  ["faq", "site.header.faq"],
  [{ url: CHANGELOG_URL }, "site.footer.changelog"],
  ["privacy", "footer.privacy"],
  ["terms", "site.footer.terms"],
  ["report", "report.button"],
];

/** A catalogue line with one {placeholder} turned into a link. */
function linked(
  text: string,
  placeholder: string,
  to: { name: string; url: string },
) {
  const [before, after = ""] = text.split(`{${placeholder}}`);
  return (
    <>
      {before}
      <a href={to.url}>{to.name}</a>
      {after}
    </>
  );
}

export function SiteFooter() {
  const t = useT();
  const locale = useLocale();
  return (
    <footer className="footer">
      <nav className="footer-nav" aria-label={t("site.footer.navLabel")}>
        {LINKS.map(([to, key]) =>
          typeof to === "string" ? (
            <Link key={key} to={pathFor(to, locale)}>
              {t(key)}
            </Link>
          ) : (
            <a key={key} href={to.url}>
              {t(key)}
            </a>
          ),
        )}
      </nav>
      <p className="made-by">
        {linked(t("site.footer.copyright"), "author", AUTHOR)} ·{" "}
        {linked(t("site.footer.operator"), "operator", OPERATOR)}
        {" · "}
        <a href={SOURCE_URL}>{t("site.footer.source")}</a>
      </p>
    </footer>
  );
}
