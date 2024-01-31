import React from 'react'
import { DocsThemeConfig } from 'nextra-theme-docs'

const config: DocsThemeConfig = {
   head: (
        <>
          <meta property="og:title" content="Interchaintest - Strangelove" />
          <meta
              property="og:description"
              content={"IBC testing environment"}
          />
          <title>{"Interchaintest - Strangelove"}</title>
          <link rel="icon" type="image/png" href="favicon.png" />
        </>
    ),
   logo: (
    <>
      <img
        src="https://strange.love/assets/press-kit/sl-logo.png"
        width="25px"
      ></img><span className="cursor-default"> &nbsp;&nbsp;Interchaintest</span>
      </>
      ),
  project: {
    link: 'https://github.com/strangelove-ventures/interchaintest',
  },
  docsRepositoryBase: 'https://github.com/strangelove-ventures/interchaintest/tree/docs-site',
  footer: {
    text: 'Built with ❤️ by Strangelove',
  },
}

export default config
