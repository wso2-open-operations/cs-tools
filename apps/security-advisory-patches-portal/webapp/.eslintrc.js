'use strict';

// Single resolve root so IDE ESLint never loads a second copy of
// eslint-plugin-react from another node_modules tree (e.g. global react-app-rewired).
module.exports = {
  root: true,
  extends: [
    require.resolve('eslint-config-react-app', { paths: [__dirname] }),
  ],
};
