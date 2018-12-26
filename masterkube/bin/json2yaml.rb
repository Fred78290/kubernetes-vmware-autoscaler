#!/usr/bin/env ruby
require 'yaml'
require 'json'
require 'fileutils'

ARGV.each do |arg|
    next unless File.exist?(arg)

    input = arg
    output = arg[0..-(File.extname(arg).length + 1)] + '.yaml'
    body = JSON.parse(File.read(input))

    File.open(output, 'w') do |f|
        f.write(body.to_yaml)
    end
end
