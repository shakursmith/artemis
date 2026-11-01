# Arrivals API Examples

This directory contains example JSON responses for the Arrivals API defined in `contracts/arrivals.yaml`.

## Event State Examples

These examples show the three states of an arrival event as the customer moves relative to a beacon.

### Event States

- **event_entered.json** - Customer device first detected the beacon at "near" proximity
- **event_dwelling.json** - Customer remains in range, moved closer to "immediate" proximity
- **event_exited.json** - Customer moved to "far" proximity or lost beacon signal

The sequence represents a typical customer visit: entering the location, browsing products near the beacon, then leaving.

## Customer Profile Examples

These examples show what the kiosk sees at different sharing levels.

### Sharing Levels

- **profile_minimal.json** - Default sharing level, shows only first name
- **profile_standard.json** - Shows first name + purchase history + loyalty status
- **profile_full.json** - Shows all available fields including review count
- **profile_new_customer.json** - A customer with no purchase history at this business

### New Customer Detection

When `purchaseHistory` and `loyaltyStatus` are both absent (as in `profile_new_customer.json`), the kiosk should recognize this as a first-time visitor and show a welcome message or prompt to leave a review.

## Customer Sharing Settings Examples

These examples show how customers configure their privacy settings.

### Sharing Settings

- **sharing_settings_default.json** - Conservative default for new customers (minimal sharing, no blocked businesses)
- **sharing_settings_standard.json** - Customer upgraded to standard sharing level
- **sharing_settings_with_blocked_business.json** - Customer blocked a specific business from seeing their data

### Privacy Model

- Customers start with **minimal** sharing by default (first name only)
- Customers can upgrade to **standard** (adds history + loyalty) or **full** (adds review count)
- Customers can block specific businesses even with a non-minimal sharing level
- Blocked businesses receive a 404 when requesting the customer profile
