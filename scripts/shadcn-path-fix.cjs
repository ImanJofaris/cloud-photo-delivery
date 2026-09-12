// shadcn CLI 4.20/4.21 resolves wildcard workspace aliases with
// `path.replace(/\.[^/]+$/, "")` to strip a file extension. On Windows the
// character class excludes only "/", so a dot anywhere in an ancestor
// directory (e.g. a username like "UF-Iman.Jofaris") is treated as the
// extension and the resolved path is truncated. Rewriting the regex to also
// exclude "\\" keeps the intended extension-stripping behavior.
const brokenSource = "\\.[^/]+$"
const fixedSource = "\\.[^/\\\\]+$"

const originalReplace = String.prototype.replace
String.prototype.replace = function (pattern, ...rest) {
  if (pattern instanceof RegExp && pattern.source === brokenSource) {
    return originalReplace.call(
      this,
      new RegExp(fixedSource, pattern.flags),
      ...rest
    )
  }
  return originalReplace.call(this, pattern, ...rest)
}

const originalTest = RegExp.prototype.test
RegExp.prototype.test = function (value) {
  if (this.source === brokenSource) {
    return new RegExp(fixedSource, this.flags).test(value)
  }
  return originalTest.call(this, value)
}
