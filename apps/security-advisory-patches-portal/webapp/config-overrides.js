const path = require('path');
const paths = require('react-scripts/config/paths');

function hasJsxRuntime() {
  if (process.env.DISABLE_NEW_JSX_TRANSFORM === 'true') {
    return false;
  }
  try {
    require.resolve('react/jsx-runtime', { paths: [paths.appPath] });
    return true;
  } catch {
    return false;
  }
}

module.exports = function override(config, env) {
  config.resolve = {
    ...config.resolve,
    alias: {
      ...config.resolve.alias,
      '@src': path.resolve(__dirname, 'src'),
    },
  };

  const eslintPlugin = config.plugins.find(
    (p) => p.constructor && p.constructor.name === 'ESLintWebpackPlugin'
  );
  if (eslintPlugin) {
    // CRA merges webpack baseConfig with package.json eslintConfig; if those
    // resolve eslint-config-react-app from different installs, ESLint 8 reports
    // Plugin "react" was conflicted between ...
    eslintPlugin.options.useEslintrc = false;
    eslintPlugin.options.resolvePluginsRelativeTo = paths.appPath;
    eslintPlugin.options.baseConfig = {
      extends: [
        require.resolve('eslint-config-react-app', { paths: [paths.appPath] }),
      ],
      rules: {
        ...(!hasJsxRuntime() && {
          'react/react-in-jsx-scope': 'error',
        }),
      },
    };
  }

  return config;
};

