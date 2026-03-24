#!/usr/bin/perl
use strict;
use warnings;

# Literal string replacement (handles &, $, /, newlines in secrets).
sub replace_all {
  my ( $s, $from, $to ) = @_;
  my $lf = length($from);
  my $lt = length($to);
  my $pos = 0;
  while ( ( my $i = index( $s, $from, $pos ) ) >= 0 ) {
    substr( $s, $i, $lf ) = $to;
    $pos = $i + $lt;
  }
  return $s;
}

my $template = $ARGV[0] || $ENV{NAKAMA_CONFIG_TEMPLATE} || '/nakama/templates/config.template.yml';
my $out      = $ARGV[1] || $ENV{NAKAMA_RUNTIME_CONFIG}   || '/tmp/nakama-runtime.yml';

my $k = $ENV{NAKAMA_SERVER_KEY}           // 'nebula-strike-dev-key';
my $u = $ENV{NAKAMA_CONSOLE_USERNAME}     // 'admin';
my $p = $ENV{NAKAMA_CONSOLE_PASSWORD}     // 'password';

open my $fh, '<', $template or die "open $template: $!";
my $t = do { local $/; <$fh> };
close $fh;

$t = replace_all( $t, '__NAKAMA_SERVER_KEY__',           $k );
$t = replace_all( $t, '__NAKAMA_CONSOLE_USERNAME__',     $u );
$t = replace_all( $t, '__NAKAMA_CONSOLE_PASSWORD__',     $p );

open my $outfh, '>', $out or die "open $out: $!";
print $outfh $t;
close $outfh;
