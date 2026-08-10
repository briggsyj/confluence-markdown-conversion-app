class Page


  constructor: (fullPath, @formatter, @utils) ->
    @path = fullPath
    @init()


  init: () ->
    @fileName = @utils.getBasename @path
    @fileBaseName = @utils.getBasename @path, '.html'
    @confluenceId = @utils.getConfluenceIdFromName(@fileBaseName)
    @filePlainText = @utils.readFile @path
    @$ = @formatter.load @filePlainText
    @content = @$.root()
    @heading = @getHeading()
    @fileNameNew = @getFileNameNew()
    @space = @utils.getBasename @utils.getDirname @path
    @spacePath = @getSpacePath()


  getSpacePath: () ->
    '../' + @utils.sanitizeFilename(@space) + '/' + @fileNameNew

  getFileNameNewRaw: () ->
    return 'index' if @fileName == 'index.html'
    @utils.sanitizeFilename(@heading)
  
  getFileNameNew: () ->
    return @getFileNameNewRaw() + '.md'

  getHeading: () ->
    title = @content.find('title').text()
    if @fileName == 'index.html'
      title
    else
      indexName = @content.find('#breadcrumbs .first').text().trim()
      title.replace indexName + ' : ', ''

  getLocalDir: () ->
    return @formatter.getLocalDir(@content)

  getIndexSpace: () ->
    # this is only useful if the page is index.html
    return @content.find('th:contains("Key")').next().text()


  ###*
  # Converts HTML file at given path to MD formatted text.
  # @return {string} Content of a file parsed to MD
  ###
  getTextToConvert: () ->
    content = @formatter.getRightContentByFileName @content, @fileName
    content = @formatter.fixHeadline content
    content = @formatter.fixIcon content
    content = @formatter.fixEmptyLink content
    content = @formatter.fixEmptyHeading content
    content = @formatter.fixPreformattedText content
    content = @formatter.fixImageWithinSpan content
    content = @formatter.removeArbitraryElements content
    content = @formatter.fixArbitraryClasses content
    content = @formatter.fixAttachmentWraper content
    content = @formatter.fixPageLog content
    # content = @formatter.fixLocalLinks content, @space, pages
    content = @formatter.addPageHeading content, @heading
    @formatter.getHtml content


module.exports = Page
