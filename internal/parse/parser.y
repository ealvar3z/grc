%term FOR IN WHILE IF NOT TWIDDLE BANG SUBSHELL SWITCH FN
%term WORD REDIR DUP PIPE SUB
%term SIMPLE ARGLIST WORDS BRACE PAREN PCMD PIPEFD /* not used in syntax */
/* operator priorities -- lowest first */
%left IF WHILE FOR SWITCH ')' NOT
%left ANDAND OROR
%left BANG SUBSHELL
%left PIPE
%left '^'
%right '$' COUNT '"'
%left SUB
%{
package parse

var parseResult *Node

var lastword int
%}
%union {
	node *Node
	str  string
}
%type<node> rc line paren brace body cmdsa cmdsan assign epilog redir
%type<node> cmd simple first word comword keyword words
%type<node> NOT FOR IN WHILE IF TWIDDLE BANG SUBSHELL SWITCH FN
%type<node> WORD REDIR DUP PIPE
%%
rc:				{ return 1; }
|	line '\n'		{$$=$1; parseResult=$$; return 1;}
line:	cmd
|	cmdsa line		{$$=N(KSeq, $1, $2);}
body:	cmd
|	cmdsan body		{$$=N(KSeq, $1, $2);}
cmdsa:	cmd ';'			{$$=$1;}
|	cmd '&'			{$$=N(KBg, $1, nil);}
cmdsan:	cmdsa
|	cmd '\n'		{$$=$1;}
brace:	'{' body '}'		{$$=N(KBrace, $2, nil);}
paren:	'(' body ')'		{$$=N(KParen, $2, nil);}
assign:	first '=' word		{$$=N(KAssign, $1, $3);}
epilog:				{$$=nil;}
|	redir epilog		{$$=L(KRedir, $1, $2);}
redir:	REDIR word		{$$=N(KRedir, $1, $2);}
|	DUP			{$$=$1;}
cmd:				{$$=nil;}
|	brace epilog		{$$=N(KCall, $1, $2);}
|	IF paren cmd
				{$$=N(Kind(IF), $2, $3);}
|	IF NOT cmd		{$$=N(Kind(IF), $2, $3);}
|	FOR '(' word IN words ')' cmd
				{$$=N(Kind(FOR), L(KWords, $3, $5), $7);}
|	FOR '(' word ')' cmd
				{$$=N(Kind(FOR), $3, $5);}
|	WHILE paren cmd
				{$$=N(Kind(WHILE), $2, $3);}
|	SWITCH word brace
				{$$=N(KSwitch, $2, $3);}
|	simple			{$$=N(KCall, $1, nil);}
|	TWIDDLE word words	{$$=N(Kind(TWIDDLE), $2, $3);}
|	cmd ANDAND cmd		{$$=N(KAnd, $1, $3);}
|	cmd OROR cmd		{$$=N(KOr, $1, $3);}
|	cmd '|' cmd		{$$=N(KPipe, $1, $3);}
|	redir cmd  %prec BANG	{$$=N(KRedir, $1, $2);}
|	assign cmd %prec BANG	{$$=N(KAssign, $1, $2);}
|	BANG cmd		{$$=N(Kind(BANG), $2, nil);}
|	SUBSHELL cmd		{$$=N(KSubshell, $2, nil);}
|	FN words brace		{$$=N(KFnDef, $2, $3);}
|	FN words		{$$=N(KFn, $2, nil);}
simple:	first
|	simple word		{$$=L(KArgList, $1, $2);}
|	simple redir		{$$=L(KArgList, $1, $2);}
first:	comword
|	first '^' word		{$$=N(KConcat, $1, $3);}
word:	keyword			{lastword=1; $$=$1; if $$ != nil { $$.Kind = KWord; }}
|	comword
|	word '^' word		{$$=N(KConcat, $1, $3);}
comword: '$' word		{$$=N(KDollar, $2, nil);}
|	'$' word SUB words ')'	{$$=N(KSub, $2, $4);}
|	'"' word		{$$=N(KQuote, $2, nil);}
|	COUNT word		{$$=N(KCount, $2, nil);}
|	WORD
|	'`' brace		{$$=N(KBackquote, $2, nil);}
|	'(' words ')'		{$$=N(KParen, $2, nil);}
|	REDIR brace		{$$=N(KRedir, $1, $2);}
keyword: FOR|IN|WHILE|IF|NOT|TWIDDLE|BANG|SUBSHELL|SWITCH|FN
words:				{$$=nil;}
|	words word		{$$=L(KWords, $1, $2);}
%%
