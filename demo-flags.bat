@echo off
REM goldpath Feature Flag Demo Script for Windows
REM This script demonstrates the 10%% rollout feature of goldpath

echo ============================================
echo   goldpath Feature Flag Rollout Demo
echo ============================================
echo.

set BASE_URL=http://localhost:8080
set FLAG_KEY=new-payment-flow

echo Step 1: Creating a feature flag with 10%% rollout...
echo.

REM Create the feature flag with 10%% rollout
curl -s -X POST "%BASE_URL%/api/v1/flags" -H "Content-Type: application/json" -d "{\"key\": \"%FLAG_KEY%\", \"name\": \"New Payment Flow\", \"description\": \"Redesigned payment processing experience\", \"enabled\": true, \"rollout\": 10.0}"

echo.
echo Step 2: Testing rollout with 20 different users...
echo We expect approximately 10%% (2 out of 20) to have the feature enabled.
echo.
echo -----------------------------------------------

set enabled_count=0
set disabled_count=0

for %%i in (1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20) do (
    set user_id=user%%i
    call :test_user %%i
)

echo -----------------------------------------------
echo.
echo Step 3: Results Summary
echo ----------------------
echo Users with feature ENABLED: %enabled_count%
echo Users with feature DISABLED: %disabled_count%
echo Total users tested: 20
echo.

set /a percentage=enabled_count * 100 / 20
echo Actual rollout percentage: %percentage%%

echo.
echo Step 4: Viewing all flags...
echo.
curl -s "%BASE_URL%/api/v1/flags"

echo.
echo Step 5: Checking metrics endpoint...
echo.
curl -s "%BASE_URL%/metrics" | findstr "flag_evaluation"

echo.
echo ============================================
echo   Demo Complete!
echo ============================================
echo.
echo You can also try updating the rollout to 50%%:
echo   curl -X PUT "%%BASE_URL%%/api/v1/flags/%%FLAG_KEY%%" -H "Content-Type: application/json" -d "{\"rollout\": 50.0}"
echo.
echo Or toggling the flag:
echo   curl -X PATCH "%%BASE_URL%%/api/v1/flags/%%FLAG_KEY%%/toggle"
echo.

goto :eof

:test_user
setlocal
set user_id=user%1
for /f "delims=" %%a in ('curl -s "%BASE_URL%/api/v1/flags/%FLAG_KEY%/evaluate?user_id=%user_id%"') do set result=%%a
echo Result: %result%
echo %result% | findstr /C:"\"enabled\":true" >nul
if %errorlevel%==0 (
    echo User %user_id%: ENABLED
    endlocal & set /a enabled_count+=1
) else (
    echo User %user_id%: disabled
    endlocal & set /a disabled_count+=1
)
goto :eof
