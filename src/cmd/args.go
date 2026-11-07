package cmd

import (
	"errors"
	"regexp"
	"strings"

	"amcd/i18n"
	"amcd/mirror"
)

// dlArgs dl 命令解析结果 / parsed dl arguments
type dlArgs struct {
	Loader        string
	LoaderVersion string
	Path          string
	Source        string
	Candidates    []string // 待用版本清单判定的版本号候选 / version-ish tokens
}

var (
	// 形如 E:\mc 或 E:/mc 的路径参数 / a drive path argument
	drivePath = regexp.MustCompile(`^[A-Za-z]:[\\/]`)
	// 版本号形态：1.20.1 / 25w43a / 24w14potato / 47.4.6 / b1.7.3
	versionLike = regexp.MustCompile(`^[0-9][A-Za-z0-9._+\-]*$`)
)

// parseDL 解析 dl 的位置参数，五个参数顺序任意
// parseDL splits the positional arguments; order does not matter
func parseDL(raw []string) (*dlArgs, error) {
	out := &dlArgs{}
	badParam := errors.New(i18n.T("err.badParam"))
	// 路径含空格且未加引号时，后续片段并入路径 / merge unquoted path fragments
	var leftovers []string
	pathSeenAt := -1

	for idx, token := range raw {
		token = strings.Trim(token, `"'`)
		if token == "" {
			continue
		}
		if !strings.HasPrefix(token, "-") {
			// Readme 里 quilt 少了减号，这里宽松接受裸加载器名
			// the spec writes "quilt" without a dash; accept a bare loader name too
			if mirror.IsLoader(token) {
				if out.Loader != "" {
					return nil, badParam
				}
				out.Loader = token
				continue
			}
			if versionLike.MatchString(token) {
				out.Candidates = append(out.Candidates, token)
				continue
			}
			leftovers = append(leftovers, token)
			continue
		}

		body := token[1:]
		switch {
		case strings.HasPrefix(body, "http://"), strings.HasPrefix(body, "https://"):
			if out.Source != "" {
				return nil, badParam
			}
			out.Source = body
		case drivePath.MatchString(body):
			if out.Path != "" {
				return nil, badParam
			}
			out.Path = body
			pathSeenAt = idx
		case mirror.IsLoader(body):
			if out.Loader != "" {
				return nil, badParam // 加载器参数只能有一个
			}
			out.Loader = body
		case versionLike.MatchString(body):
			out.Candidates = append(out.Candidates, body)
		default:
			return nil, badParam
		}
	}

	if len(leftovers) > 0 {
		if pathSeenAt < 0 {
			return nil, badParam
		}
		out.Path = out.Path + " " + strings.Join(leftovers, " ")
	}
	return out, nil
}

// splitVersions 用版本清单区分 MC 版本与加载器版本
// splitVersions uses the manifest to tell the MC version from the loader version
func splitVersions(candidates []string, known func(string) bool) (mcVersion, loaderVersion string, err error) {
	var rest []string
	for _, c := range candidates {
		if mcVersion == "" && known(c) {
			mcVersion = c
			continue
		}
		rest = append(rest, c)
	}
	if len(rest) > 1 {
		return "", "", errors.New(i18n.T("err.badParam"))
	}
	if len(rest) == 1 {
		loaderVersion = rest[0]
	}
	return mcVersion, loaderVersion, nil
}

// splitArgs 按空格切分，支持双引号与单引号（交互模式用）
// splitArgs splits a line on spaces while honouring quotes
func splitArgs(s string) []string {
	var out []string
	var cur strings.Builder
	var quote rune
	inToken := false
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote = r
			inToken = true
		case r == ' ' || r == '\t':
			if inToken {
				out = append(out, cur.String())
				cur.Reset()
				inToken = false
			}
		default:
			cur.WriteRune(r)
			inToken = true
		}
	}
	if inToken {
		out = append(out, cur.String())
	}
	return out
}
