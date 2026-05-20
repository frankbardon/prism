-- Drop into your init.lua (or a file under lua/plugins/) to wire Prism
-- schema validation via vscode-json-languageserver. Requires
-- nvim-lspconfig + mason (or manual install of vscode-json-languageserver).
-- Schema path assumes .prism/ sits at cwd of the running nvim.

require("lspconfig").jsonls.setup({
  settings = {
    json = {
      schemas = {
        {
          fileMatch = { "*.prism.json" },
          url = "./.prism/schemas/spec.schema.json",
        },
      },
      validate = { enable = true },
    },
  },
})
