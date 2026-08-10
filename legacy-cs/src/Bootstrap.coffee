class Bootstrap

  _fs = require 'fs'
  _exec = require 'sync-exec'
  _path = require 'path'
  _ncp = require 'ncp'
  _cheerio = require 'cheerio'
  _mkdirp = require 'mkdirp'
  _turndownService = require 'turndown'
  _turndownPluginGfm = require 'joplin-turndown-plugin-gfm'
  _confluenceTurndownPluginGfm = require 'turndown-plugin-confluence-to-gfm'
  _figlet = require 'figlet'

  Utils = require './Utils'
  Logger = require './Logger'
  Formatter = require './Formatter'
  App = require './App'
  PageFactory = require './PageFactory'


  ###*
  # @param {string} pathResource Directory with HTML files or one file. Can be nested.
  # @param {string|void} pathResult Directory where MD files will be generated to. Current dir will be used if none given.
  ###
  run: (pathResource, pathResult = '', runScript = false, verbose = false, confluenceUrl) ->
    pathResource = _path.resolve pathResource
    pathResult = _path.resolve pathResult

    logger = new Logger Logger.INFO
    utils = new Utils _fs, _path, _ncp, logger
    formatter = new Formatter _cheerio, utils, logger
    pageFactory = new PageFactory formatter, utils
    app = new App _fs, _exec, _path, _mkdirp, utils, formatter, pageFactory, logger, _turndownService, _turndownPluginGfm, _confluenceTurndownPluginGfm, verbose

    logger.info _figlet.textSync 'say', {
      font: 'Standard',
      horizontalLayout: 'default',
      verticalLayout: 'default',
      width: 80,
      whitespaceBreak: true
    }
    logger.info _figlet.textSync 'goodbye', {
      font: 'Big Money-ne',
      horizontalLayout: 'default',
      verticalLayout: 'default',
      width: 80,
      whitespaceBreak: true
    }
    logger.info _figlet.textSync 'to', {
      font: 'Standard',
      horizontalLayout: 'default',
      verticalLayout: 'default',
      width: 80,
      whitespaceBreak: true
    }
    logger.info _figlet.textSync 'confluence', {
      font: 'Bloody',
      horizontalLayout: 'default',
      verticalLayout: 'default',
      width: 100,
      whitespaceBreak: true
    }

    logger.info '\n----------------------------------------'
    logger.info 'Using source: ' + pathResource
    logger.info 'Using destination: ' + pathResult
    logger.info '----------------------------------------\n'

    app.convert pathResource, pathResult, runScript, confluenceUrl

    logger.info _figlet.textSync 'export complete', {
      font: 'Standard',
      horizontalLayout: 'default',
      verticalLayout: 'default',
      width: 100,
      whitespaceBreak: true
    }


module.exports = Bootstrap
