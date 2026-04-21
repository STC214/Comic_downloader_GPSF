$path = '.\browser\chromium_session_playwright.go'
$text = Get-Content -Path $path -Raw
$text = [regex]::Replace($text, '(?m)^.*NoViewport = playwright\.Bool\(true\).*$' , '// Keep viewport unset so resizing/maximizing the window also resizes the page.`r`ncontextOptions.NoViewport = playwright.Bool(true)')
$text = [regex]::Replace($text, '(?m)^.*if strings\.TrimSpace\(text\) == "100%" \{.*$' , '// Prefer a real mouse click on the reader 100% button.`r`nif strings.TrimSpace(text) == "100%" {')
$text = $text.Replace('return "", errors.New("invalid page")', 'return errors.New("invalid page")')
$text = $text.Replace('return errors.New("invalid page")', 'return "", errors.New("invalid page")')
$text = $text.Replace('return "", errors.New("invalid page")', 'return errors.New("invalid page")')
Set-Content -Path $path -Value $text -Encoding utf8
