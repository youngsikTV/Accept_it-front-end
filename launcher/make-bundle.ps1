# ACCEPT IT - 설치 없이 실행되는 폴더 만들기
# 이 컴퓨터(인터넷 연결 필요)에서 한 번만 실행하면 dist\AcceptIt 폴더가 만들어집니다.
# 그 폴더(또는 AcceptIt.zip)를 다른 사람에게 주면, Docker/Java 설치 없이 AcceptIt.exe 만 눌러 실행할 수 있습니다.
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
Add-Type -AssemblyName System.IO.Compression.FileSystem

$here = Split-Path -Parent $MyInvocation.MyCommand.Path
# launcher 폴더에서 위로 올라가며 build.gradle 이 있는 프로젝트 폴더를 찾습니다.
$root = Split-Path -Parent $here
for ($i = 0; $i -lt 5 -and -not (Test-Path (Join-Path $root 'build.gradle')); $i++) {
  $parent = Split-Path -Parent $root
  if (-not $parent -or $parent -eq $root) { break }
  $root = $parent
}
if (-not (Test-Path (Join-Path $root 'build.gradle'))) {
  throw "프로젝트 폴더(build.gradle 이 있는 곳)를 찾지 못했습니다. launcher 폴더를 그 폴더 안에 두세요. (시작 위치: $here)"
}
Write-Host "프로젝트 폴더: $root"
$out = Join-Path $root 'dist\AcceptIt'
$dl  = Join-Path $root 'dist\_download'
$rt  = Join-Path $out 'runtime'
New-Item -ItemType Directory -Force $dl, $rt | Out-Null

function Get-File($urls, $dest, $minMB) {
  if ((Test-Path $dest) -and ((Get-Item $dest).Length -gt $minMB * 1MB)) { Write-Host "  이미 받아 둔 파일 사용: $dest"; return }
  foreach ($u in $urls) {
    try {
      Write-Host "  내려받는 중: $u"
      Invoke-WebRequest -Uri $u -OutFile $dest -UseBasicParsing
      if ((Get-Item $dest).Length -gt $minMB * 1MB) { return }
    } catch { Write-Host "  실패: $($_.Exception.Message)" }
  }
  throw "내려받기에 실패했습니다. 브라우저로 직접 받아 $dest 이름으로 저장한 뒤 다시 실행하세요. (사용법.txt 참고)"
}
function Expand-First($zip, $tmp, $target) {
  if (Test-Path $tmp) { Remove-Item $tmp -Recurse -Force }
  [System.IO.Compression.ZipFile]::ExtractToDirectory($zip, $tmp)
  $inner = Get-ChildItem $tmp -Directory | Select-Object -First 1
  if (Test-Path $target) { Remove-Item $target -Recurse -Force }
  Move-Item $inner.FullName $target
  Remove-Item $tmp -Recurse -Force
}

Write-Host "`n[1/5] 백엔드 jar 만들기 (처음에는 몇 분 걸립니다)"
Push-Location $root
& "$root\gradlew.bat" bootJar -x test
if ($LASTEXITCODE -ne 0) { Pop-Location; throw "gradlew bootJar 실패 (JDK 21 이 필요합니다)" }
Pop-Location
$jar = Get-ChildItem (Join-Path $root 'build\libs') -Filter *.jar | Where-Object { $_.Name -notlike '*-plain.jar' } |
       Sort-Object LastWriteTime -Descending | Select-Object -First 1
if (-not $jar) { throw "build\libs 에 jar 가 없습니다." }
Copy-Item $jar.FullName (Join-Path $rt 'app.jar') -Force
Write-Host "  app.jar <- $($jar.Name)"

Write-Host "`n[2/5] Java 21 (JRE) 준비"
$jreZip = Join-Path $dl 'jre.zip'
Get-File @('https://api.adoptium.net/v3/binary/latest/21/ga/windows/x64/jre/hotspot/normal/eclipse') $jreZip 30
Expand-First $jreZip (Join-Path $dl 'jre_tmp') (Join-Path $rt 'jre')

Write-Host "`n[3/5] MySQL 8.0 (설치 없는 ZIP 버전) 준비"
$myZip = Join-Path $dl 'mysql.zip'
Get-File @(
  'https://cdn.mysql.com/archives/mysql-8.0/mysql-8.0.40-winx64.zip',
  'https://dev.mysql.com/get/Downloads/MySQL-8.0/mysql-8.0.40-winx64.zip'
) $myZip 100
Expand-First $myZip (Join-Path $dl 'mysql_tmp') (Join-Path $rt 'mysql')
$my = Join-Path $rt 'mysql'
Remove-Item (Join-Path $my 'docs'), (Join-Path $my 'include') -Recurse -Force -ErrorAction SilentlyContinue
Get-ChildItem $my -Recurse -Include *.pdb, *.lib, mysqld-debug.exe -ErrorAction SilentlyContinue | Remove-Item -Force -ErrorAction SilentlyContinue

Write-Host "`n[3.5/5] cloudflared 준비 (와이파이가 달라도 접속하는 '인터넷 공개' 기능용, 실패해도 계속 진행)"
try {
  $cf = Join-Path $dl 'cloudflared.exe'
  Get-File @('https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-amd64.exe') $cf 5
  Copy-Item $cf (Join-Path $rt 'cloudflared.exe') -Force
} catch { Write-Host "  cloudflared 를 받지 못했습니다. 같은 와이파이에서만 쓸 수 있습니다. ($($_.Exception.Message))" }

Write-Host "`n[4/5] 실행 파일 복사"
Copy-Item (Join-Path $here 'AcceptIt.exe') $out -Force
Copy-Item (Join-Path $here 'items.txt') (Join-Path $out 'items.txt') -Force
Copy-Item (Join-Path $here 'AcceptIt-public.bat') $out -Force
Copy-Item (Join-Path $here 'keywords.txt') (Join-Path $out 'keywords.txt') -Force

Write-Host "`n[5/5] 압축 (AcceptIt.zip)"
$zip = Join-Path $root 'dist\AcceptIt.zip'
if (Test-Path $zip) { Remove-Item $zip -Force }
[System.IO.Compression.ZipFile]::CreateFromDirectory($out, $zip, [System.IO.Compression.CompressionLevel]::Optimal, $true)

Write-Host "`n완료!"
Write-Host "  폴더 : $out"
Write-Host "  압축 : $zip  ($([math]::Round((Get-Item $zip).Length / 1MB)) MB)"
Write-Host "이 폴더(또는 zip)를 전달하세요. 받은 사람은 C:\AcceptIt 처럼 영문 경로에 풀고 AcceptIt.exe 를 실행하면 됩니다."
