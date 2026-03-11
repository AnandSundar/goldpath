#!/bin/bash

# goldpath Feature Flag Demo Script
# This script demonstrates the 10% rollout feature of goldpath

echo "============================================"
echo "  goldpath Feature Flag Rollout Demo"
echo "============================================"
echo ""

# Configuration
BASE_URL="http://localhost:8080"
FLAG_KEY="new-payment-flow"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "Step 1: Creating a feature flag with 10% rollout..."
echo ""

# Create the feature flag with 10% rollout
curl -s -X POST "$BASE_URL/api/v1/flags" \
  -H "Content-Type: application/json" \
  -d '{
    "key": "'"$FLAG_KEY"'",
    "name": "New Payment Flow",
    "description": "Redesigned payment processing experience",
    "enabled": true,
    "rollout": 10.0
  }' | jq '.'

echo ""
echo "Step 2: Testing rollout with 20 different users..."
echo "We expect approximately 10% (2 out of 20) to have the feature enabled."
echo ""
echo "-----------------------------------------------"

enabled_count=0
disabled_count=0

for i in {1..20}; do
  user_id="user$i"
  
  # Call the evaluate endpoint with user_id query parameter
  result=$(curl -s "$BASE_URL/api/v1/flags/$FLAG_KEY/evaluate?user_id=$user_id")
  
  # Extract enabled value
  enabled=$(echo "$result" | jq -r '.data.enabled')
  returned_user_id=$(echo "$result" | jq -r '.data.user_id')
  
  if [ "$enabled" = "true" ]; then
    echo -e "User $returned_user_id: ${GREEN}ENABLED${NC} ✓"
    enabled_count=$((enabled_count + 1))
  else
    echo -e "User $returned_user_id: ${RED}disabled${NC}"
    disabled_count=$((disabled_count + 1))
  fi
done

echo "-----------------------------------------------"
echo ""
echo "Step 3: Results Summary"
echo "----------------------"
echo -e "Users with feature ENABLED: ${GREEN}$enabled_count${NC}"
echo -e "Users with feature DISABLED: ${RED}$disabled_count${NC}"
echo "Total users tested: $((enabled_count + disabled_count))"
echo ""

# Calculate percentage
percentage=$((enabled_count * 100 / 20))
echo "Actual rollout percentage: ${percentage}%"

if [ $enabled_count -ge 1 ] && [ $enabled_count -le 4 ]; then
  echo -e "${GREEN}✓ Rollout is working correctly! (~10% of users)${NC}"
else
  echo -e "${YELLOW}Note: With small sample sizes, variance is expected${NC}"
fi

echo ""
echo "Step 4: Viewing all flags..."
echo ""
curl -s "$BASE_URL/api/v1/flags" | jq '.'

echo ""
echo "Step 5: Checking metrics endpoint..."
echo ""
curl -s "$BASE_URL/metrics" | grep "flag_evaluation" | head -5

echo ""
echo "============================================"
echo "  Demo Complete!"
echo "============================================"
echo ""
echo "You can also try:"
echo "  - Updating rollout: curl -X PUT '$BASE_URL/api/v1/flags/$FLAG_KEY' \\"
echo "      -H 'Content-Type: application/json' \\"
echo "      -d '{\"rollout\": 50.0}'"
echo ""
echo "  - Toggling flag: curl -X PATCH '$BASE_URL/api/v1/flags/$FLAG_KEY/toggle'"
echo ""
