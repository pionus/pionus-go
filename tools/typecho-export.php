<?php
/**
 * Read-only Typecho exporter for pionus-go migration.
 *
 * Run it from the Typecho root (or set TYPECHO_ROOT):
 *
 *   php typecho-export.php > typecho.json
 *
 * It prints normalized JSON on stdout. It never writes to the database.
 */

if (PHP_SAPI !== 'cli') {
    fwrite(STDERR, "This script must be run from the command line.\n");
    exit(1);
}

$root = getenv('TYPECHO_ROOT') ?: getcwd();
$config = rtrim($root, '/') . '/config.inc.php';
if (!is_file($config)) {
    fwrite(STDERR, "config.inc.php not found under {$root}\n");
    exit(1);
}

require $config;

$db = Typecho_Db::get();

function rows($db, $table, $order = null)
{
    $select = $db->select()->from('table.' . $table);
    if ($order !== null) {
        $select->order($order, Typecho_Db::SORT_ASC);
    }
    return $db->fetchAll($select);
}

function optionValue($options, $name, $user = null)
{
    foreach ($options as $row) {
        if ($row['name'] !== $name) {
            continue;
        }
        if ($user !== null && (string)$row['user'] !== (string)$user) {
            continue;
        }
        return $row['value'];
    }
    return null;
}

$options = rows($db, 'options');
$contents = rows($db, 'contents', 'cid');
$metas = rows($db, 'metas', 'mid');
$relationships = rows($db, 'relationships');
$comments = rows($db, 'comments', 'coid');
$users = rows($db, 'users', 'uid');

$fields = array();
try {
    $fields = rows($db, 'fields');
} catch (Exception $e) {
    // Older Typecho installs may not have the fields table.
}

$site = array(
    'url' => optionValue($options, 'siteUrl'),
    'title' => optionValue($options, 'title'),
    'description' => optionValue($options, 'description'),
    'keywords' => optionValue($options, 'keywords'),
    'rewrite' => (int)optionValue($options, 'rewrite'),
    'theme' => optionValue($options, 'theme'),
    'timezone' => (int)optionValue($options, 'timezone'),
    'lang' => optionValue($options, 'lang'),
    'page_size' => (int)optionValue($options, 'pageSize'),
    'posts_list_size' => (int)optionValue($options, 'postsListSize'),
    'comments_list_size' => (int)optionValue($options, 'commentsListSize'),
    'comments_require_moderation' => (int)optionValue($options, 'commentsRequireModeration'),
    'comments_threaded' => (int)optionValue($options, 'commentsThreaded'),
    'comments_max_nesting' => (int)optionValue($options, 'commentsMaxNestingLevels'),
    'comments_order' => optionValue($options, 'commentsOrder'),
    'comments_anti_spam' => (int)optionValue($options, 'commentsAntiSpam'),
    'default_allow_comment' => (int)optionValue($options, 'defaultAllowComment'),
    'generator' => optionValue($options, 'generator'),
);

$out = array(
    'generated_at' => gmdate('c'),
    'site' => $site,
    'contents' => $contents,
    'metas' => $metas,
    'relationships' => $relationships,
    'comments' => $comments,
    'users' => array_map(function ($u) {
        // Password hashes are Typecho phpass values and are intentionally
        // omitted; reset the account with -create-admin after importing.
        unset($u['password']);
        return $u;
    }, $users),
    'fields' => $fields,
    'options' => $options,
);

$json = json_encode($out, JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES | JSON_PRETTY_PRINT);
if ($json === false) {
    fwrite(STDERR, 'json_encode failed: ' . json_last_error_msg() . "\n");
    exit(1);
}

echo $json, "\n";
