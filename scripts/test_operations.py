import os,unittest
from unittest.mock import patch,MagicMock
import deploy,smoke
class Operations(unittest.TestCase):
 def setUp(self):
  self.env=patch.dict(os.environ,{"DATABASE_URL":"postgresql://test","RENDER_DEPLOY_HOOK_URL":"https://api.render.com/deploy/test?key=private","VERCEL_DEPLOY_HOOK_URL":"https://api.vercel.com/v1/integrations/deploy/test/private","EXPECTED_REVISION":"a"*40,"BACKUP_AGE_RECIPIENT":"age1test"});self.env.start();self.addCleanup(self.env.stop)
 def test_rejects_foreign_deploy_host(self):
  os.environ['RENDER_DEPLOY_HOOK_URL']='https://attacker.example/';self.assertRaises(ValueError,deploy.configuration)
 def test_backend_targets_tested_revision(self):
  with patch('deploy.urllib.request.urlopen') as opener:
   opener.return_value.__enter__.return_value.status=200
   deploy.run('backend')
   self.assertIn('ref='+'a'*40,opener.call_args.args[0].full_url)
 def test_missing_secret_fails_closed(self):
  del os.environ['DATABASE_URL'];self.assertRaises(ValueError,deploy.configuration)
 def test_smoke_refuses_public_catalog_data(self):
  with patch('smoke.request',side_effect=[(200,b'{"status":"ok"}',{}),(200,b'[]',{})]):self.assertRaises(ValueError,smoke.check,'https://api','')
 def test_smoke_checks_revision(self):
  with patch('smoke.request',return_value=(200,b'{"status":"ok","revision":"old"}',{})):self.assertRaises(ValueError,smoke.check,'https://api','','new')

 def test_smoke_waits_for_matching_frontend_revision(self):
  import urllib.error
  responses=[(200,b'{"status":"ok","revision":"new"}',{}),urllib.error.HTTPError('https://api',401,'Unauthorized',{},None),(200,b'<div id="root"></div>',{}),(200,b'{"revision":"old"}',{})]
  with patch('smoke.request',side_effect=responses):self.assertRaises(ValueError,smoke.check,'https://api','https://web','new')
