# MCP Market Feature Documentation

This document introduces the **MCP Market** feature in the management backend: how to access third-party MCP markets, aggregate discovery services, import service configurations and incorporate them into the system's global MCP service list.

Related documents:

- [MCP Architecture Documentation](./mcp.md)
- [Management Console Guide](./manager_console_guide.md)

---

## 1. Feature Positioning

MCP Market is used to solve the problem of "low efficiency in accessing remote MCP services", supporting:

- Configure multiple MCP market connections (such as ModelScope, etc.)
- Aggregate service catalogs from multiple markets
- View service details (endpoint, transport protocol, etc.)
- One-click import service configuration to this system
- Enable/disable/edit/delete imported services

Imported services will participate in the system global MCP service configuration merge (co-effective with manually configured MCP services).

---

## 2. Role Permissions and Entry Points

Role permissions:

- Only administrators can operate

Management backend entry points:

- `Administrator -> MCP Market`

Page contains two tabs:

- `Market Discovery`
- `Imported Services`

---

## 3. Core Concepts

### 3.1 MCP Market (Market)

Represents a "MCP market catalog source that can be accessed", containing:

- Market name
- Provider identifier (provider)
- Catalog URL (catalog_url)
- Detail URL template (detail_url_template, optional)
- Auth Token (optional)
- Enable status

### 3.2 Aggregated Service List

The system will pull service catalogs from enabled markets and aggregate display, supporting:

- Search service name/description/Service ID
- View details
- Import configuration

When some markets fail to pull, the page will display a "some markets failed to pull" warning list, without affecting other market result display.

### 3.3 Imported Services

Imported services form independent configuration items in this system, can directly participate in runtime MCP service connection. Support configuration:

- Name
- Transport type (`sse` / `streamablehttp`)
- URL
- Headers (JSON)
- Source market and provider identifier (optional metadata)
- Enable status

---

## 4. Common Operation Flow (Administrator)

## 4.1 Add MCP Market Connection

Click `Add Connection` in `Market Discovery` tab, fill in:

- `Provider`: Preferably select built-in provider preset (will automatically fill catalog URL template)
- `Name`
- `Catalog URL`
- `Detail URL Template` (optional)
- `Enable`
- `Token` (if market requires)

It is recommended to perform connection test (see below) before saving and using.

## 4.2 Test Market Connection

Click `Test` in market list operation menu:

- Success will return "number of discoverable services"
- Failure will prompt catalog connection/auth error

Suitable for troubleshooting:

- Token invalid
- Catalog URL error
- Market temporarily unavailable

## 4.3 Browse and Search Aggregated Services

In `Aggregated Service List` area you can:

- Enter keywords to search services
- Paginate through aggregated results
- Click `Details` to view service endpoint information

Service detail page usually includes:

- Service name
- Source market
- Service ID
- Description
- Endpoint list (transport protocol + URL)

## 4.4 One-click Import Service Configuration (Recommended)

Click `Import Service Configuration and Hot Update` in service detail popup:

- System will generate one or more import service configurations based on service details
- After successful import, "Imported Services" list will refresh
- Page will switch to `Imported Services` tab

"Hot Update" means after importing configuration, it can immediately participate in runtime service set (no backend restart needed).

## 4.5 Manually Add/Edit Imported Service

In `Imported Services` tab you can click `Add Service` to manually enter, or edit imported items.

Key field descriptions:

- `Transport`: Currently supports `SSE`, `StreamableHTTP`
- `URL`: Remote MCP service entry
- `Headers(JSON)`: Used to carry auth information, such as `Authorization`
- `Enable`: After disabling, will not participate in runtime available service set

`Headers(JSON)` must be JSON object, for example:

```json
{
  "Authorization": "Bearer <token>"
}
```

---

## 5. Relationship with Global MCP Configuration

MCP Market is not a replacement for the `MCP Configuration` page, but a supplementary source.

Runtime available global MCP service set comes from two parts merged:

- Global services manually maintained by administrator in `MCP Configuration` page
- Services imported from MCP Market and enabled

Therefore recommended practice is:

1. Use MCP Market for quick discovery and import
2. Enable and select services as needed in `MCP Configuration` / Agent

---

## 6. API (Backend Interfaces)

The following are management-related interfaces (require administrator permission):

### 6.1 Market Connection Management

- `GET /admin/mcp-markets`
- `POST /admin/mcp-markets`
- `PUT /admin/mcp-markets/:id`
- `DELETE /admin/mcp-markets/:id`
- `POST /admin/mcp-markets/:id/test`

### 6.2 Market Discovery and Details

- `GET /admin/mcp-market/providers`
- `GET /admin/mcp-market/services`
- `GET /admin/mcp-market/services/:market_id/*service_id`
- `POST /admin/mcp-market/import`

### 6.3 Imported Service Management

- `GET /admin/mcp-market/imported-services`
- `POST /admin/mcp-market/imported-services`
- `PUT /admin/mcp-market/imported-services/:id`
- `DELETE /admin/mcp-market/imported-services/:id`

---

## 7. FAQ and Troubleshooting

### 7.1 Aggregated list is empty

Troubleshooting order:

1. Check if market connection is enabled
2. Perform "test" on that market
3. Check if Token is valid
4. Check if Catalog URL / Detail URL Template is correct

### 7.2 Import successful but service not visible at runtime

Common causes:

- Imported service is disabled
- Global MCP master switch is off
- Agent configured `mcp_service_names`, and does not include this service name

### 7.3 What happens if Token is left blank when editing market?

Leaving Token blank in edit popup usually means "do not modify existing Token" (interface will display current masked status prompt).

---

## 8. Usage Suggestions

- Preferably use built-in provider presets to reduce issues caused by catalog interface field differences
- After importing services that need long-term stable use, unify naming for easy agent selection by name
- For production environment remote services, recommend using `Headers(JSON)` to configure auth, and do token rotation
