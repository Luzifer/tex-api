# 1.0.0 / 2023-09-07

  * Breaking: Add support for default env & raw TeX post, unpack received zip
  * Refactor & add support for direct PDF download
  * Add new URL parameters
  * Lint: Fix linter errors
  * Update deps

> [!WARNING]  
> This release breaks the previous approach of writing the zip file to the file system and letting the script unpack the files itself. You might need to adjust your build-script if you are not using the default script included in the container.

# 0.4.1 / 2023-08-30

  * Update dependencies

# 0.4.0 / 2022-03-03

  * Use atomic operations to update status
  * Update image location
  * Update dependencies
  * Remove vendoring

# 0.3.0 / 2019-07-06

  * Fix broken error handling
  * Update vendored packages
  * Switch from dep to go mod
  * Update Alpine, lower image size

# 0.2.0 / 2018-09-17

  * Remove never implemented api docs
  * Add more verbose processing logs
  * Read parameters from ENV
  * Improve error handling
  * Update imports

# 0.1.0 / 2018-09-17

  * Migrate to Alpine as the base image
  * Switch to `dep` for vendoring, update deps
  * Fix: Deliver correct content-type
  * Simplify executor script

# 0.0.0 / 2017-03-06

  * Initial version
