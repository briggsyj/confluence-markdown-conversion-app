Bootstrap = require './Bootstrap'

inputPath = process.argv[2]
outputPath = process.argv[3]
confluenceUrl = process.argv[4]
runScript = if (!process.argv[5] || process.argv[4] == 'false') then false else true
verboseLogs = if (!process.argv[6] || process.argv[5] == 'false') then false else true

bootstrap = new Bootstrap
bootstrap.run inputPath, outputPath, runScript, verboseLogs, confluenceUrl
