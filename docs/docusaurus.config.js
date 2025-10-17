// @ts-check
import {themes as prismThemes} from 'prism-react-renderer';

/** @type {import('@docusaurus/types').Config} */
const config = {
  title: 'S-57 Parser',
  tagline: 'Go library for parsing S-57 Electronic Navigational Charts',
  favicon: 'img/favicon.ico',
  url: process.env.DOCUSAURUS_URL || 'https://beetlebugorg.github.io/',
  baseUrl: process.env.DOCUSAURUS_BASE_URL || '/s57/',
  organizationName: 'beetlebugorg',
  projectName: 's57',
  onBrokenLinks: 'throw',
  onBrokenMarkdownLinks: 'warn',
  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },
  presets: [
    [
      'classic',
      ({
        docs: {
          routeBasePath: '/',
          sidebarPath: './sidebars.js',
          editUrl:
            'https://github.com/beetlebugorg/s57/tree/main/docs/',
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      }),
    ],
  ],
  themeConfig: ({
    image: 'img/s57.jpg',
    navbar: {
      title: 'S-57 Parser',
      items: [
        {
          href: 'https://github.com/beetlebugorg/s57',
          position: 'right',
          className: 'header-github-link',
          'aria-label': 'GitHub repository',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [],
      copyright: `Copyright © ${new Date().getFullYear()} Jeremy Collins. MIT License.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      additionalLanguages: ['bash', 'go'],
    },
  }),
};

export default config;
