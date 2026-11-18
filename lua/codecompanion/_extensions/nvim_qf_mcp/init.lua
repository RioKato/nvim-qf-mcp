local M = {}

function M.setup(opts)
    opts = opts or {}
    local config = require("codecompanion.config")
    local name = "nvim-qf-mcp"

    config.mcp.servers[name] = function()
        return {
            cmd = { opts.command or "nvim-qf-mcp", "-socket", vim.v.servername },
        }
    end

    local defaults = config.mcp.opts.default_servers
    if not vim.tbl_contains(defaults, name) then
        table.insert(defaults, name)
    end
end

return M
