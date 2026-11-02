# Remove Favorite Item Example

This example shows how to remove an item from a customer's favorites list.

## Request

```
DELETE /api/customer/c1d2e3f4-a5b6-7890-cdef-123456789012/favorites/i1t2e3m4-i5d6-7890-item-123456789012
```

**Path Parameters:**
- `customerId`: c1d2e3f4-a5b6-7890-cdef-123456789012
- `itemId`: i1t2e3m4-i5d6-7890-item-123456789012

**Headers:**
```
Authorization: Bearer <access_token>
```

## Response

```
204 No Content
```

The DELETE operation is idempotent. Attempting to remove an item that is not in the favorites list will also return 204.
